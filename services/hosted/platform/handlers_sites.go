package platform

import (
	"bytes"
	"context"
	"crypto/md5" // #nosec G501 -- md5 used for content hash (ETags), not security
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/forumline/forumline/services/hosted/oapi"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type siteManifest struct {
	Files   map[string]siteFileEntry `json:"files"`
	Updated time.Time                `json:"updated"`
}

type siteFileEntry struct {
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	ETag        string `json:"etag"`
	Updated     string `json:"updated"`
}

var allowedExtensions = map[string]bool{
	".html": true, ".css": true, ".js": true, ".json": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".svg": true, ".webp": true, ".ico": true,
	".woff": true, ".woff2": true, ".ttf": true,
	".txt": true, ".xml": true,
}

var extToContentType = map[string]string{
	".html":  "text/html; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".js":    "application/javascript; charset=utf-8",
	".json":  "application/json; charset=utf-8",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".svg":   "image/svg+xml",
	".webp":  "image/webp",
	".ico":   "image/x-icon",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".txt":   "text/plain; charset=utf-8",
	".xml":   "application/xml; charset=utf-8",
}

const (
	maxFileSize    = 5 << 20 // 5MB per file
	metaObjectName = "_meta.json"
)

func (h *Handlers) r2Client() (*minio.Client, error) {
	endpoint := fmt.Sprintf("%s.r2.cloudflarestorage.com", h.R2Account)
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(h.R2KeyID, h.R2Secret, ""),
		Secure: true,
	})
}

// authenticateOwnerCtx looks up the tenant and verifies the caller owns it.
// Returns the tenant on success, or an error string + HTTP status code on failure.
func (h *Handlers) authenticateOwnerCtx(ctx context.Context, slug string) (*Tenant, string, int) {
	forumlineID := ForumlineIDFromContext(ctx)
	if forumlineID == "" {
		return nil, "authentication required", http.StatusUnauthorized
	}
	tenant := h.Store.BySlug(slug)
	if tenant == nil {
		return nil, "forum not found", http.StatusNotFound
	}
	if tenant.OwnerForumlineID != forumlineID {
		return nil, "not authorized", http.StatusForbidden
	}
	return tenant, "", 0
}

func sanitizePath(p string) (string, error) {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.Contains(p, "..") {
		return "", fmt.Errorf("path traversal not allowed")
	}
	if strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("absolute paths not allowed")
	}
	// No hidden files
	for _, segment := range strings.Split(p, "/") {
		if strings.HasPrefix(segment, ".") {
			return "", fmt.Errorf("hidden files not allowed")
		}
	}
	// Check extension
	ext := path.Ext(p)
	if ext == "" || !allowedExtensions[ext] {
		return "", fmt.Errorf("file type %q not allowed", ext)
	}
	return path.Clean(p), nil
}

func r2Key(slug, filePath string) string {
	return fmt.Sprintf("sites/%s/files/%s", slug, filePath)
}

func r2MetaKey(slug string) string {
	return fmt.Sprintf("sites/%s/%s", slug, metaObjectName)
}

// loadManifest fetches the manifest from R2.
func (h *Handlers) loadManifest(ctx context.Context, client *minio.Client, slug string) (*siteManifest, error) {
	obj, err := client.GetObject(ctx, h.R2Bucket, r2MetaKey(slug), minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get manifest object: %w", err)
	}
	defer func() { _ = obj.Close() }()
	data, err := io.ReadAll(obj)
	if err != nil {
		// Object doesn't exist yet — return empty manifest (not an error)
		return &siteManifest{Files: make(map[string]siteFileEntry)}, nil
	}
	var m siteManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}
	if m.Files == nil {
		m.Files = make(map[string]siteFileEntry)
	}
	return &m, nil
}

// saveManifest writes the manifest to R2.
func (h *Handlers) saveManifest(ctx context.Context, client *minio.Client, slug string, m *siteManifest) error {
	m.Updated = time.Now().UTC()
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = client.PutObject(ctx, h.R2Bucket, r2MetaKey(slug), bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	return err
}

// updateSiteState updates has_custom_site and site_storage_bytes in the DB.
func (h *Handlers) updateSiteState(ctx context.Context, slug string, manifest *siteManifest) error {
	_, hasIndex := manifest.Files["index.html"]
	var totalBytes int64
	for _, f := range manifest.Files {
		totalBytes += f.Size
	}
	_, err := h.Pool.Exec(ctx,
		`UPDATE platform_tenants SET has_custom_site = $1, site_storage_bytes = $2 WHERE slug = $3`,
		hasIndex, totalBytes, slug)
	if err != nil {
		return err
	}
	return h.Store.Refresh(ctx)
}

// ListOwnedSites returns the domains and slugs of forums owned by the caller.
// GET /api/platform/owned-sites
func (h *Handlers) ListOwnedSites(ctx context.Context, _ oapi.ListOwnedSitesRequestObject) (oapi.ListOwnedSitesResponseObject, error) {
	forumlineID := ForumlineIDFromContext(ctx)
	if forumlineID == "" {
		return oapi.ListOwnedSites401TextResponse("authentication required"), nil
	}
	var sites oapi.ListOwnedSites200JSONResponse
	for _, t := range h.Store.All() {
		if t.OwnerForumlineID == forumlineID {
			sites = append(sites, oapi.OwnedSite{Domain: t.Domain, Slug: t.Slug})
		}
	}
	if sites == nil {
		sites = oapi.ListOwnedSites200JSONResponse{}
	}
	return sites, nil
}

// ListFiles returns the manifest for a forum's custom site.
// GET /api/platform/sites/{slug}/files
func (h *Handlers) ListFiles(ctx context.Context, request oapi.ListFilesRequestObject) (oapi.ListFilesResponseObject, error) {
	if _, errMsg, status := h.authenticateOwnerCtx(ctx, request.Slug); errMsg != "" {
		switch status {
		case http.StatusUnauthorized:
			return oapi.ListFiles401JSONResponse{Error: errMsg}, nil
		case http.StatusNotFound:
			return oapi.ListFiles404JSONResponse{Error: errMsg}, nil
		default:
			return oapi.ListFiles403JSONResponse{Error: errMsg}, nil
		}
	}

	client, err := h.r2Client()
	if err != nil {
		return oapi.ListFiles500JSONResponse{Error: "storage unavailable"}, nil
	}
	manifest, err := h.loadManifest(ctx, client, request.Slug)
	if err != nil {
		return oapi.ListFiles500JSONResponse{Error: "failed to load manifest"}, nil
	}

	tenant := h.Store.BySlug(request.Slug)
	files := make(map[string]oapi.SiteFileEntry, len(manifest.Files))
	for k, v := range manifest.Files {
		t, _ := time.Parse(time.RFC3339, v.Updated)
		files[k] = oapi.SiteFileEntry{
			Size:        v.Size,
			ContentType: v.ContentType,
			Etag:        v.ETag,
			Updated:     t,
		}
	}
	return oapi.ListFiles200JSONResponse{
		Files:        files,
		Updated:      manifest.Updated,
		StorageBytes: tenant.SiteStorageBytes,
		StorageLimit: tenant.SiteStorageLimit,
	}, nil
}

// GetFile downloads a specific file.
// GET /api/platform/sites/{slug}/files/{path...}
func (h *Handlers) GetFile(ctx context.Context, request oapi.GetFileRequestObject) (oapi.GetFileResponseObject, error) {
	if request.Path == "" {
		return oapi.GetFile400JSONResponse{Error: "invalid path"}, nil
	}
	if _, errMsg, status := h.authenticateOwnerCtx(ctx, request.Slug); errMsg != "" {
		switch status {
		case http.StatusUnauthorized:
			return oapi.GetFile401JSONResponse{Error: errMsg}, nil
		case http.StatusNotFound:
			return oapi.GetFile404JSONResponse{Error: errMsg}, nil
		default:
			return oapi.GetFile403JSONResponse{Error: errMsg}, nil
		}
	}

	filePath, err := sanitizePath(request.Path)
	if err != nil {
		return oapi.GetFile400JSONResponse{Error: err.Error()}, nil
	}

	client, err := h.r2Client()
	if err != nil {
		return oapi.GetFile500JSONResponse{Error: "storage unavailable"}, nil
	}
	obj, err := client.GetObject(ctx, h.R2Bucket, r2Key(request.Slug, filePath), minio.GetObjectOptions{})
	if err != nil {
		return oapi.GetFile404JSONResponse{Error: "file not found"}, nil
	}

	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return oapi.GetFile404JSONResponse{Error: "file not found"}, nil
	}

	return oapi.GetFile200ApplicationoctetStreamResponse{
		Body:          obj,
		ContentLength: info.Size,
		Headers: oapi.GetFile200ResponseHeaders{
			ContentType:   info.ContentType,
			ContentLength: fmt.Sprintf("%d", info.Size),
		},
	}, nil
}

// PutFile creates or updates a file.
// PUT /api/platform/sites/{slug}/files/{path...}
func (h *Handlers) PutFile(ctx context.Context, request oapi.PutFileRequestObject) (oapi.PutFileResponseObject, error) {
	if request.Path == "" {
		return oapi.PutFile400JSONResponse{Error: "invalid path"}, nil
	}
	tenant, errMsg, status := h.authenticateOwnerCtx(ctx, request.Slug)
	if errMsg != "" {
		switch status {
		case http.StatusUnauthorized:
			return oapi.PutFile401JSONResponse{Error: errMsg}, nil
		case http.StatusNotFound:
			return oapi.PutFile404JSONResponse{Error: errMsg}, nil
		default:
			return oapi.PutFile403JSONResponse{Error: errMsg}, nil
		}
	}

	filePath, err := sanitizePath(request.Path)
	if err != nil {
		return oapi.PutFile400JSONResponse{Error: err.Error()}, nil
	}

	// Read body with size limit (request.Body is already r.Body from strict handler)
	limited := io.LimitReader(request.Body, maxFileSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return oapi.PutFile413JSONResponse{Error: "file too large (max 5MB)"}, nil
	}
	if int64(len(data)) > maxFileSize {
		return oapi.PutFile413JSONResponse{Error: "file too large (max 5MB)"}, nil
	}

	client, err := h.r2Client()
	if err != nil {
		return oapi.PutFile500JSONResponse{Error: "storage unavailable"}, nil
	}

	// Load manifest and check quota
	manifest, err := h.loadManifest(ctx, client, request.Slug)
	if err != nil {
		return oapi.PutFile500JSONResponse{Error: "failed to load manifest"}, nil
	}

	newSize := int64(len(data))
	currentUsage := tenant.SiteStorageBytes
	// Subtract old file size if replacing
	if old, exists := manifest.Files[filePath]; exists {
		currentUsage -= old.Size
	}
	if currentUsage+newSize > tenant.SiteStorageLimit {
		return oapi.PutFile413JSONResponse{Error: "storage quota exceeded"}, nil
	}

	// Determine content type
	ext := path.Ext(filePath)
	contentType := extToContentType[ext]
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Compute ETag
	hash := md5.Sum(data) // #nosec G401 -- md5 used for content hash (ETags), not security
	etag := hex.EncodeToString(hash[:])

	// Upload to R2
	_, err = client.PutObject(ctx, h.R2Bucket, r2Key(request.Slug, filePath), bytes.NewReader(data), newSize, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		log.Printf("R2 upload error for %s/%s: %v", request.Slug, filePath, err) // #nosec G706 -- slug is validated before use
		return oapi.PutFile500JSONResponse{Error: "upload failed"}, nil
	}

	// Update manifest
	manifest.Files[filePath] = siteFileEntry{
		Size:        newSize,
		ContentType: contentType,
		ETag:        etag,
		Updated:     time.Now().UTC().Format(time.RFC3339),
	}
	// Update DB first (easier to recover if manifest save fails)
	if err := h.updateSiteState(ctx, request.Slug, manifest); err != nil {
		log.Printf("failed to update site state for %s: %v", request.Slug, err) // #nosec G706 -- slug is validated before use
	}

	if err := h.saveManifest(ctx, client, request.Slug, manifest); err != nil {
		log.Printf("failed to save manifest for %s: %v", request.Slug, err) // #nosec G706 -- slug is validated before use
	}

	// Invalidate cache
	if h.SiteCache != nil {
		h.SiteCache.Invalidate(request.Slug, filePath)
	}

	return oapi.PutFile200JSONResponse{
		Path: filePath,
		Etag: etag,
		Size: fmt.Sprintf("%d", newSize),
	}, nil
}

// DeleteFile deletes a file from the custom site.
// DELETE /api/platform/sites/{slug}/files/{path...}
func (h *Handlers) DeleteFile(ctx context.Context, request oapi.DeleteFileRequestObject) (oapi.DeleteFileResponseObject, error) {
	if request.Path == "" {
		return oapi.DeleteFile400JSONResponse{Error: "invalid path"}, nil
	}
	if _, errMsg, status := h.authenticateOwnerCtx(ctx, request.Slug); errMsg != "" {
		switch status {
		case http.StatusUnauthorized:
			return oapi.DeleteFile401JSONResponse{Error: errMsg}, nil
		case http.StatusNotFound:
			return oapi.DeleteFile404JSONResponse{Error: errMsg}, nil
		default:
			return oapi.DeleteFile403JSONResponse{Error: errMsg}, nil
		}
	}

	filePath, err := sanitizePath(request.Path)
	if err != nil {
		return oapi.DeleteFile400JSONResponse{Error: err.Error()}, nil
	}

	client, err := h.r2Client()
	if err != nil {
		return oapi.DeleteFile500JSONResponse{Error: "storage unavailable"}, nil
	}

	// Delete from R2
	if err := client.RemoveObject(ctx, h.R2Bucket, r2Key(request.Slug, filePath), minio.RemoveObjectOptions{}); err != nil {
		log.Printf("R2 delete error for %s/%s: %v", request.Slug, filePath, err) // #nosec G706 -- slug is validated before use
	}

	// Update manifest
	manifest, _ := h.loadManifest(ctx, client, request.Slug)
	delete(manifest.Files, filePath)
	if err := h.saveManifest(ctx, client, request.Slug, manifest); err != nil {
		log.Printf("failed to save manifest for %s: %v", request.Slug, err) // #nosec G706 -- slug is validated before use
	}

	// Update DB state
	if err := h.updateSiteState(ctx, request.Slug, manifest); err != nil {
		log.Printf("failed to update site state for %s: %v", request.Slug, err) // #nosec G706 -- slug is validated before use
	}

	// Invalidate cache
	if h.SiteCache != nil {
		h.SiteCache.Invalidate(request.Slug, filePath)
	}

	return oapi.DeleteFile200JSONResponse{Deleted: filePath}, nil
}

// MultipartUpload handles drag-and-drop multipart file uploads.
// POST /api/platform/sites/{slug}/upload
func (h *Handlers) MultipartUpload(ctx context.Context, request oapi.MultipartUploadRequestObject) (oapi.MultipartUploadResponseObject, error) {
	tenant, errMsg, status := h.authenticateOwnerCtx(ctx, request.Slug)
	if errMsg != "" {
		switch status {
		case http.StatusUnauthorized:
			return oapi.MultipartUpload401JSONResponse{Error: errMsg}, nil
		case http.StatusNotFound:
			return oapi.MultipartUpload404JSONResponse{Error: errMsg}, nil
		default:
			return oapi.MultipartUpload403JSONResponse{Error: errMsg}, nil
		}
	}

	client, err := h.r2Client()
	if err != nil {
		return oapi.MultipartUpload500JSONResponse{Error: "storage unavailable"}, nil
	}

	manifest, err := h.loadManifest(ctx, client, request.Slug)
	if err != nil {
		return oapi.MultipartUpload500JSONResponse{Error: "failed to load manifest"}, nil
	}

	currentUsage := tenant.SiteStorageBytes
	var uploaded []string
	var errors []string

	// request.Body is a *multipart.Reader provided by the strict handler
	for {
		part, err := request.Body.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("read part error: %s", err.Error()))
			break
		}

		filename := part.FileName()
		if filename == "" {
			_ = part.Close()
			continue
		}

		filePath, err := sanitizePath(filename)
		if err != nil {
			_ = part.Close()
			errors = append(errors, fmt.Sprintf("%s: %s", filename, err.Error()))
			continue
		}

		// Read with size limit
		limited := io.LimitReader(part, maxFileSize+1)
		data, err := io.ReadAll(limited)
		_ = part.Close()
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: failed to read", filePath))
			continue
		}
		if int64(len(data)) > maxFileSize {
			errors = append(errors, fmt.Sprintf("%s: file too large (max 5MB)", filePath))
			continue
		}

		// Check quota
		oldSize := int64(0)
		if old, exists := manifest.Files[filePath]; exists {
			oldSize = old.Size
		}
		if currentUsage-oldSize+int64(len(data)) > tenant.SiteStorageLimit {
			errors = append(errors, fmt.Sprintf("%s: storage quota exceeded", filePath))
			continue
		}

		ext := path.Ext(filePath)
		contentType := extToContentType[ext]
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		_, err = client.PutObject(ctx, h.R2Bucket, r2Key(request.Slug, filePath), bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
			ContentType: contentType,
		})
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: upload failed", filePath))
			continue
		}

		hash := md5.Sum(data) // #nosec G401 -- md5 used for content hash (ETags), not security
		manifest.Files[filePath] = siteFileEntry{
			Size:        int64(len(data)),
			ContentType: contentType,
			ETag:        hex.EncodeToString(hash[:]),
			Updated:     time.Now().UTC().Format(time.RFC3339),
		}
		currentUsage = currentUsage - oldSize + int64(len(data))
		uploaded = append(uploaded, filePath)

		if h.SiteCache != nil {
			h.SiteCache.Invalidate(request.Slug, filePath)
		}
	}

	if err := h.saveManifest(ctx, client, request.Slug, manifest); err != nil {
		log.Printf("failed to save manifest for %s: %v", request.Slug, err) // #nosec G706 -- slug is validated before use
	}
	if err := h.updateSiteState(ctx, request.Slug, manifest); err != nil {
		log.Printf("failed to update site state for %s: %v", request.Slug, err) // #nosec G706 -- slug is validated before use
	}

	if uploaded == nil {
		uploaded = []string{}
	}
	if errors == nil {
		errors = []string{}
	}

	return oapi.MultipartUpload200JSONResponse{
		Uploaded: uploaded,
		Errors:   errors,
	}, nil
}

// ResetSite deletes all custom files and reverts to default SPA.
// POST /api/platform/sites/{slug}/reset
func (h *Handlers) ResetSite(ctx context.Context, request oapi.ResetSiteRequestObject) (oapi.ResetSiteResponseObject, error) {
	if _, errMsg, status := h.authenticateOwnerCtx(ctx, request.Slug); errMsg != "" {
		switch status {
		case http.StatusUnauthorized:
			return oapi.ResetSite401JSONResponse{Error: errMsg}, nil
		case http.StatusNotFound:
			return oapi.ResetSite404JSONResponse{Error: errMsg}, nil
		default:
			return oapi.ResetSite403JSONResponse{Error: errMsg}, nil
		}
	}

	client, err := h.r2Client()
	if err != nil {
		return oapi.ResetSite500JSONResponse{Error: "storage unavailable"}, nil
	}

	manifest, _ := h.loadManifest(ctx, client, request.Slug)

	// Delete all files from R2
	for filePath := range manifest.Files {
		if err := client.RemoveObject(ctx, h.R2Bucket, r2Key(request.Slug, filePath), minio.RemoveObjectOptions{}); err != nil {
			log.Printf("R2 delete error during reset for %s/%s: %v", request.Slug, filePath, err) // #nosec G706 -- slug is validated before use
		}
		if h.SiteCache != nil {
			h.SiteCache.Invalidate(request.Slug, filePath)
		}
	}

	// Delete manifest
	if err := client.RemoveObject(ctx, h.R2Bucket, r2MetaKey(request.Slug), minio.RemoveObjectOptions{}); err != nil {
		log.Printf("R2 delete manifest error for %s: %v", request.Slug, err) // #nosec G706 -- slug is validated before use
	}

	// Update DB
	emptyManifest := &siteManifest{Files: make(map[string]siteFileEntry)}
	if err := h.updateSiteState(ctx, request.Slug, emptyManifest); err != nil {
		log.Printf("failed to update site state for %s: %v", request.Slug, err) // #nosec G706 -- slug is validated before use
	}

	return oapi.ResetSite200JSONResponse{Status: oapi.ResetComplete}, nil
}

// RegisterRoutes registers all platform API routes on the given mux.
//
// We don't use oapi.HandlerFromMux directly because the generated code registers
// file routes as {path} (single segment) instead of {path...} (wildcard). Since
// file paths can contain subdirectories (e.g. css/style.css), we need the
// catch-all pattern. This function registers the correct patterns.
func RegisterRoutes(mux *http.ServeMux, si oapi.StrictServerInterface) {
	// Wrap the strict implementation as a ServerInterface via the generated adapter.
	// ForumlineIDMiddleware injects X-Forumline-ID into the context so strict handlers
	// can retrieve it without touching *http.Request directly.
	wrapper := oapi.ServerInterfaceWrapper{
		Handler: oapi.NewStrictHandler(si, nil),
		HandlerMiddlewares: []oapi.MiddlewareFunc{
			ForumlineIDMiddleware,
		},
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		},
	}

	// Routes without path params — identical to generated code
	mux.HandleFunc("GET /api/platform/forums", wrapper.ListForums)
	mux.HandleFunc("POST /api/platform/forums", wrapper.ProvisionForum)
	mux.HandleFunc("GET /api/platform/forums/{slug}/export", wrapper.ExportForum)
	mux.HandleFunc("GET /api/platform/owned-sites", wrapper.ListOwnedSites)
	mux.HandleFunc("GET /api/platform/sites/{slug}/files", wrapper.ListFiles)
	mux.HandleFunc("POST /api/platform/sites/{slug}/reset", wrapper.ResetSite)
	mux.HandleFunc("POST /api/platform/sites/{slug}/upload", wrapper.MultipartUpload)

	// File routes need {path...} for subdirectory support (e.g. css/style.css).
	// The generated code uses {path} which only matches a single segment.
	mux.HandleFunc("DELETE /api/platform/sites/{slug}/files/{path...}", wrapper.DeleteFile)
	mux.HandleFunc("GET /api/platform/sites/{slug}/files/{path...}", wrapper.GetFile)
	mux.HandleFunc("PUT /api/platform/sites/{slug}/files/{path...}", wrapper.PutFile)
}

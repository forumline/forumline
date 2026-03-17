package forum

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/forumline/forumline/backend/auth"
)

// HandleAvatarUpload accepts a multipart file upload and stores it via FileStorage.
// POST /api/avatars/upload
func (h *Handlers) HandleAvatarUpload(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	// 5 MB max
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		http.Error(w, "File too large (max 5MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file field", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}

	if !strings.HasPrefix(contentType, "image/") {
		http.Error(w, "Only image files allowed", http.StatusBadRequest)
		return
	}

	path := r.FormValue("path")
	if path == "" {
		path = fmt.Sprintf("user/%s/custom.png", userID)
	}
	userPrefix := fmt.Sprintf("user/%s/", userID)
	if !strings.HasPrefix(path, userPrefix) && !strings.HasPrefix(path, "thread/") {
		http.Error(w, "Invalid path", http.StatusForbidden)
		return
	}

	publicURL, err := h.Config.Storage.Upload(r.Context(), path, file, header.Size, contentType)
	if err != nil {
		if _, ok := err.(*StorageDisabledError); ok {
			http.Error(w, "File uploads are not configured", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "Upload failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": publicURL})
}

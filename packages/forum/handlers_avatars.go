package forum

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/forumline/forumline/backend/auth"
	"github.com/forumline/forumline/forum/oapi"
)

// UploadAvatar accepts a multipart file upload and stores it via FileStorage.
// POST /api/avatars/upload
func (h *Handlers) UploadAvatar(ctx context.Context, request oapi.UploadAvatarRequestObject) (oapi.UploadAvatarResponseObject, error) {
	userID := auth.UserIDFromContext(ctx)

	// request.Body is a *multipart.Reader; iterate parts to find "file" and "path"
	mr := request.Body
	var (
		fileData    []byte
		contentType string
		fileSize    int64
		pathVal     string
	)

	for {
		part, err := mr.NextPart()
		if err != nil {
			break // io.EOF or done
		}
		formName := part.FormName()
		switch formName {
		case "file":
			contentType = part.Header.Get("Content-Type")
			if contentType == "" {
				contentType = "image/png"
			}
			buf := make([]byte, 32*1024)
			for {
				n, readErr := part.Read(buf)
				if n > 0 {
					fileData = append(fileData, buf[:n]...)
					fileSize += int64(n)
				}
				if readErr != nil {
					break
				}
			}
		case "path":
			pathBuf := make([]byte, 1024)
			n, _ := part.Read(pathBuf)
			pathVal = strings.TrimSpace(string(pathBuf[:n]))
		}
		_ = part.Close()
	}

	if len(fileData) == 0 {
		return oapi.UploadAvatar400TextResponse("Missing file field"), nil
	}

	if !strings.HasPrefix(contentType, "image/") {
		return oapi.UploadAvatar400TextResponse("Only image files allowed"), nil
	}

	if len(fileData) > 5<<20 {
		return oapi.UploadAvatar400TextResponse("File too large (max 5MB)"), nil
	}

	if pathVal == "" {
		pathVal = fmt.Sprintf("user/%s/custom.png", userID)
	}
	userPrefix := fmt.Sprintf("user/%s/", userID)
	if !strings.HasPrefix(pathVal, userPrefix) && !strings.HasPrefix(pathVal, "thread/") {
		return oapi.UploadAvatar403TextResponse("Invalid path"), nil
	}

	publicURL, err := h.Config.Storage.Upload(ctx, pathVal, bytes.NewReader(fileData), fileSize, contentType)
	if err != nil {
		if _, ok := err.(*StorageDisabledError); ok {
			return oapi.UploadAvatar503TextResponse("File uploads are not configured"), nil
		}
		return oapi.UploadAvatar400TextResponse("Upload failed"), nil
	}

	return oapi.UploadAvatar200JSONResponse{Url: &publicURL}, nil
}

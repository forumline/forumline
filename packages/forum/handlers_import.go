package forum

import (
	"context"
	"encoding/json"

	"github.com/forumline/forumline/backend/auth"
	"github.com/forumline/forumline/forum/oapi"
)

// convertToLocalExportData converts an oapi.ExportData to the forum-local
// ExportData via a JSON round-trip. Both structs share wire-compatible JSON
// field names; this avoids duplicating all field mappings.
func convertToLocalExportData(src *oapi.ExportData) (*ExportData, error) {
	b, err := json.Marshal(src)
	if err != nil {
		return nil, err
	}
	var dst ExportData
	if err := json.Unmarshal(b, &dst); err != nil {
		return nil, err
	}
	return &dst, nil
}

// ImportData imports forum data from a forumline export file.
// POST /api/admin/import
//
// Note: request.Body is *oapi.ImportDataJSONRequestBody (= *oapi.ExportData).
// The forum-local ExportData used by Import() is a separate struct — we convert
// by re-encoding through JSON (both are wire-compatible).
func (h *Handlers) ImportData(ctx context.Context, request oapi.ImportDataRequestObject) (oapi.ImportDataResponseObject, error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == "" {
		return oapi.ImportData401JSONResponse{UnauthorizedJSONResponse: oapi.UnauthorizedJSONResponse{Error: "authentication required"}}, nil
	}

	if err := h.AdminSvc.VerifyAdmin(ctx, userID); err != nil {
		status, msg := serviceErrStatus(err)
		if status == 403 {
			return oapi.ImportData403JSONResponse{ForbiddenJSONResponse: oapi.ForbiddenJSONResponse{Error: msg}}, nil
		}
		return oapi.ImportData403JSONResponse{ForbiddenJSONResponse: oapi.ForbiddenJSONResponse{Error: msg}}, nil
	}

	oapiData := request.Body
	if !oapiData.ForumlineVersion.Valid() {
		return oapi.ImportData400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse{Error: "unsupported export version"}}, nil
	}

	// Convert oapi.ExportData → forum.ExportData via JSON round-trip.
	// Both structs share identical JSON field names; this is the cleanest
	// conversion without duplicating all fields.
	localData, err := convertToLocalExportData(oapiData)
	if err != nil {
		return oapi.ImportData400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse{Error: "invalid export file: " + err.Error()}}, nil
	}

	if err := Import(ctx, h.Store.DB, localData); err != nil {
		return oapi.ImportData500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: "import failed: " + err.Error()}}, nil
	}

	ok := true
	msg := "import complete"
	forum := &oapi.ForumMeta{
		Domain: oapiData.Forum.Domain,
		Name:   oapiData.Forum.Name,
		Slug:   oapiData.Forum.Slug,
	}
	if oapiData.Forum.Description != nil {
		forum.Description = oapiData.Forum.Description
	}
	return oapi.ImportData200JSONResponse{
		Ok:      &ok,
		Forum:   forum,
		Message: &msg,
	}, nil
}

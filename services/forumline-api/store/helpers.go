package store

import (
	"github.com/jackc/pgx/v5/pgtype"
)

// pgtextPtr converts pgtype.Text to *string (nil when not valid).
func pgtextPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// textToPgtext converts a string to pgtype.Text (always valid).
func textToPgtext(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// optTextToPgtext converts *string to pgtype.Text (nil → invalid).
func optTextToPgtext(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

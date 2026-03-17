package store

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forumline/forumline/forum/sqlcdb"
)

// --- pgUUID / uuidStr round-trip ---

func TestPgUUID_ValidUUID(t *testing.T) {
	input := "550e8400-e29b-41d4-a716-446655440000"
	u := pgUUID(input)
	if !u.Valid {
		t.Fatal("expected Valid=true")
	}
	got := uuidStr(u)
	if got != input {
		t.Errorf("round-trip failed: got %q, want %q", got, input)
	}
}

func TestPgUUID_Panics_OnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on invalid UUID")
		}
	}()
	pgUUID("not-a-uuid")
}

func TestUuidStr_InvalidReturnsEmpty(t *testing.T) {
	var u pgtype.UUID // Valid = false
	if got := uuidStr(u); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// --- pgtextPtr / textToPgtext / optTextToPgtext ---

func TestTextToPgtext(t *testing.T) {
	pt := textToPgtext("hello")
	if !pt.Valid || pt.String != "hello" {
		t.Errorf("got Valid=%v String=%q", pt.Valid, pt.String)
	}
}

func TestPgtextPtr_Valid(t *testing.T) {
	pt := pgtype.Text{String: "world", Valid: true}
	got := pgtextPtr(pt)
	if got == nil || *got != "world" {
		t.Errorf("expected pointer to %q, got %v", "world", got)
	}
}

func TestPgtextPtr_Invalid_ReturnsNil(t *testing.T) {
	pt := pgtype.Text{}
	if pgtextPtr(pt) != nil {
		t.Error("expected nil for invalid text")
	}
}

func TestOptTextToPgtext_Nil(t *testing.T) {
	pt := optTextToPgtext(nil)
	if pt.Valid {
		t.Error("expected Valid=false for nil input")
	}
}

func TestOptTextToPgtext_NonNil(t *testing.T) {
	s := "test"
	pt := optTextToPgtext(&s)
	if !pt.Valid || pt.String != "test" {
		t.Errorf("got Valid=%v String=%q", pt.Valid, pt.String)
	}
}

// --- Timestamp helpers ---

func TestPgTimestamp_RoundTrip(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 30, 0, 0, time.UTC)
	ts := pgTimestamp(now)
	got := tsStr(ts)
	want := "2025-06-15T12:30:00Z"
	if got != want {
		t.Errorf("tsStr = %q, want %q", got, want)
	}
}

func TestTsStr_Invalid(t *testing.T) {
	var ts pgtype.Timestamptz
	if got := tsStr(ts); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestTsStrPtr_Valid(t *testing.T) {
	ts := pgTimestamp(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	got := tsStrPtr(ts)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if *got != "2025-01-01T00:00:00Z" {
		t.Errorf("got %q", *got)
	}
}

func TestTsStrPtr_Invalid(t *testing.T) {
	var ts pgtype.Timestamptz
	if tsStrPtr(ts) != nil {
		t.Error("expected nil for invalid timestamp")
	}
}

func TestTsStrPtr_Zero(t *testing.T) {
	ts := pgTimestamp(time.Time{})
	// Valid=true but time is zero
	if tsStrPtr(ts) != nil {
		t.Error("expected nil for zero time")
	}
}

// --- profileFromSqlc ---

func TestProfileFromSqlc(t *testing.T) {
	id := pgUUID("550e8400-e29b-41d4-a716-446655440000")
	now := pgTimestamp(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	displayName := pgtype.Text{String: "Test User", Valid: true}

	row := sqlcdb.Profile{
		ID:          id,
		Username:    "testuser",
		DisplayName: displayName,
		AvatarUrl:   pgtype.Text{},
		Bio:         pgtype.Text{},
		Website:     pgtype.Text{},
		IsAdmin:     true,
		ForumlineID: pgtype.Text{String: "fl-123", Valid: true},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	p := profileFromSqlc(row)

	wantID := pgUUID("550e8400-e29b-41d4-a716-446655440000")
	if p.Id != [16]byte(wantID.Bytes) {
		t.Errorf("Id = %v", p.Id)
	}
	if p.Username != "testuser" {
		t.Errorf("Username = %q", p.Username)
	}
	if p.DisplayName == nil || *p.DisplayName != "Test User" {
		t.Errorf("DisplayName = %v", p.DisplayName)
	}
	if p.AvatarUrl != nil {
		t.Errorf("AvatarUrl should be nil, got %v", p.AvatarUrl)
	}
	if !p.IsAdmin {
		t.Error("expected IsAdmin=true")
	}
	if p.ForumlineId == nil || *p.ForumlineId != "fl-123" {
		t.Errorf("ForumlineId = %v", p.ForumlineId)
	}
}

// --- categoryFromThreadRow ---

func TestCategoryFromThreadRow(t *testing.T) {
	id := pgUUID("660e8400-e29b-41d4-a716-446655440000")
	now := pgTimestamp(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))
	desc := pgtype.Text{String: "A category", Valid: true}

	cat := categoryFromThreadRow(id, "General", "general", desc, 5, now)

	if cat.Name != "General" {
		t.Errorf("Name = %q", cat.Name)
	}
	if cat.Slug != "general" {
		t.Errorf("Slug = %q", cat.Slug)
	}
	if cat.Description == nil || *cat.Description != "A category" {
		t.Errorf("Description = %v", cat.Description)
	}
	if cat.SortOrder != 5 {
		t.Errorf("SortOrder = %d", cat.SortOrder)
	}
}

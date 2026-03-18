package store

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/forumline/forumline/forum/sqlcdb"
)

// --- profileFromSqlc ---

func TestProfileFromSqlc(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	displayName := "Test User"
	forumlineID := "fl-123"

	row := sqlcdb.Profile{
		ID:          id,
		Username:    "testuser",
		DisplayName: &displayName,
		AvatarUrl:   nil,
		Bio:         nil,
		Website:     nil,
		IsAdmin:     true,
		ForumlineID: &forumlineID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	p := profileFromSqlc(row)

	wantID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	if p.Id != wantID {
		t.Errorf("Id = %v, want %v", p.Id, wantID)
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
	id := uuid.MustParse("660e8400-e29b-41d4-a716-446655440000")
	now := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	desc := "A category"

	cat := categoryFromThreadRow(id, "General", "general", &desc, 5, now)

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

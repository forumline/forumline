package store

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// pgUUID converts a UUID string to pgtype.UUID.
// Panics if s is not a valid UUID — this is a programming error, not a runtime condition.
func pgUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		panic(fmt.Sprintf("pgUUID: invalid UUID %q: %v", s, err))
	}
	return u
}

// uuidStr converts pgtype.UUID to string.
func uuidStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// pgUUID2OapiUUID converts pgtype.UUID to openapi_types.UUID ([16]byte).
func pgUUID2OapiUUID(u pgtype.UUID) openapi_types.UUID {
	return openapi_types.UUID(u.Bytes)
}

// uuidPtrToOapi converts a nullable pgtype.UUID to *openapi_types.UUID.
func uuidPtrToOapi(u pgtype.UUID) *openapi_types.UUID {
	if !u.Valid {
		return nil
	}
	v := openapi_types.UUID(u.Bytes)
	return &v
}

// pgtextPtr converts pgtype.Text to *string.
func pgtextPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// textToPgtext converts string to pgtype.Text.
func textToPgtext(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// optTextToPgtext converts *string to pgtype.Text.
func optTextToPgtext(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// pgTimestamp converts time.Time to pgtype.Timestamptz.
func pgTimestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// tsStr converts pgtype.Timestamptz to RFC3339 string.
func tsStr(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}

// tsStrPtr converts pgtype.Timestamptz to *string (nil if zero).
func tsStrPtr(t pgtype.Timestamptz) *string {
	if !t.Valid || t.Time.IsZero() {
		return nil
	}
	s := t.Time.Format(time.RFC3339)
	return &s
}

// tsTime converts pgtype.Timestamptz to time.Time.
func tsTime(t pgtype.Timestamptz) time.Time {
	return t.Time
}

// tsTimePtr converts pgtype.Timestamptz to *time.Time (nil if invalid or zero).
func tsTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid || t.Time.IsZero() {
		return nil
	}
	v := t.Time
	return &v
}

// profileFromSqlc converts a sqlcdb.Profile to an oapi.Profile.
func profileFromSqlc(p sqlcdb.Profile) oapi.Profile {
	return oapi.Profile{
		Id:          pgUUID2OapiUUID(p.ID),
		Username:    p.Username,
		DisplayName: pgtextPtr(p.DisplayName),
		AvatarUrl:   pgtextPtr(p.AvatarUrl),
		Bio:         pgtextPtr(p.Bio),
		Website:     pgtextPtr(p.Website),
		IsAdmin:     p.IsAdmin,
		ForumlineId: pgtextPtr(p.ForumlineID),
		CreatedAt:   tsTime(p.CreatedAt),
		UpdatedAt:   tsTime(p.UpdatedAt),
	}
}

// authorProfileFromThreadRow extracts an oapi.Profile from thread row author fields.
func authorProfileFromThreadRow(
	id pgtype.UUID, username string, displayName, avatarURL, bio, website pgtype.Text,
	isAdmin bool, forumlineID pgtype.Text, createdAt, updatedAt pgtype.Timestamptz,
) oapi.Profile {
	return oapi.Profile{
		Id:          pgUUID2OapiUUID(id),
		Username:    username,
		DisplayName: pgtextPtr(displayName),
		AvatarUrl:   pgtextPtr(avatarURL),
		Bio:         pgtextPtr(bio),
		Website:     pgtextPtr(website),
		IsAdmin:     isAdmin,
		ForumlineId: pgtextPtr(forumlineID),
		CreatedAt:   tsTime(createdAt),
		UpdatedAt:   tsTime(updatedAt),
	}
}

// categoryFromThreadRow extracts an oapi.Category from thread row category fields.
func categoryFromThreadRow(
	id pgtype.UUID, name, slug string, description pgtype.Text, sortOrder int32, createdAt pgtype.Timestamptz,
) oapi.Category {
	return oapi.Category{
		Id:          pgUUID2OapiUUID(id),
		Name:        name,
		Slug:        slug,
		Description: pgtextPtr(description),
		SortOrder:   int(sortOrder),
		CreatedAt:   tsTime(createdAt),
	}
}

// threadRowToOapi converts a GetThreadRow to oapi.Thread.
func threadRowToOapi(r sqlcdb.GetThreadRow) oapi.Thread {
	return oapi.Thread{
		Id:         pgUUID2OapiUUID(r.ID),
		CategoryId: pgUUID2OapiUUID(r.CategoryID),
		AuthorId:   pgUUID2OapiUUID(r.AuthorID),
		Title:      r.Title,
		Slug:       r.Slug,
		Content:    pgtextPtr(r.Content),
		ImageUrl:   pgtextPtr(r.ImageUrl),
		IsPinned:   r.IsPinned,
		IsLocked:   r.IsLocked,
		ViewCount:  int(r.ViewCount),
		PostCount:  int(r.PostCount),
		LastPostAt: tsTimePtr(r.LastPostAt),
		CreatedAt:  tsTime(r.CreatedAt),
		UpdatedAt:  tsTime(r.UpdatedAt),
		Author: authorProfileFromThreadRow(
			r.AuthorID2, r.AuthorUsername, r.AuthorDisplayName, r.AuthorAvatarUrl,
			r.AuthorBio, r.AuthorWebsite, r.AuthorIsAdmin, r.AuthorForumlineID,
			r.AuthorCreatedAt, r.AuthorUpdatedAt,
		),
		Category: categoryFromThreadRow(
			r.CatID, r.CatName, r.CatSlug, r.CatDescription, r.CatSortOrder, r.CatCreatedAt,
		),
	}
}

// listThreadsRowToOapi converts a ListThreadsRow to oapi.Thread.
func listThreadsRowToOapi(r sqlcdb.ListThreadsRow) oapi.Thread {
	return oapi.Thread{
		Id:         pgUUID2OapiUUID(r.ID),
		CategoryId: pgUUID2OapiUUID(r.CategoryID),
		AuthorId:   pgUUID2OapiUUID(r.AuthorID),
		Title:      r.Title,
		Slug:       r.Slug,
		Content:    pgtextPtr(r.Content),
		ImageUrl:   pgtextPtr(r.ImageUrl),
		IsPinned:   r.IsPinned,
		IsLocked:   r.IsLocked,
		ViewCount:  int(r.ViewCount),
		PostCount:  int(r.PostCount),
		LastPostAt: tsTimePtr(r.LastPostAt),
		CreatedAt:  tsTime(r.CreatedAt),
		UpdatedAt:  tsTime(r.UpdatedAt),
		Author: authorProfileFromThreadRow(
			r.AuthorID2, r.AuthorUsername, r.AuthorDisplayName, r.AuthorAvatarUrl,
			r.AuthorBio, r.AuthorWebsite, r.AuthorIsAdmin, r.AuthorForumlineID,
			r.AuthorCreatedAt, r.AuthorUpdatedAt,
		),
		Category: categoryFromThreadRow(
			r.CatID, r.CatName, r.CatSlug, r.CatDescription, r.CatSortOrder, r.CatCreatedAt,
		),
	}
}

// listThreadsByCategoryRowToOapi converts a ListThreadsByCategoryRow to oapi.Thread.
func listThreadsByCategoryRowToOapi(r sqlcdb.ListThreadsByCategoryRow) oapi.Thread {
	return oapi.Thread{
		Id:         pgUUID2OapiUUID(r.ID),
		CategoryId: pgUUID2OapiUUID(r.CategoryID),
		AuthorId:   pgUUID2OapiUUID(r.AuthorID),
		Title:      r.Title,
		Slug:       r.Slug,
		Content:    pgtextPtr(r.Content),
		ImageUrl:   pgtextPtr(r.ImageUrl),
		IsPinned:   r.IsPinned,
		IsLocked:   r.IsLocked,
		ViewCount:  int(r.ViewCount),
		PostCount:  int(r.PostCount),
		LastPostAt: tsTimePtr(r.LastPostAt),
		CreatedAt:  tsTime(r.CreatedAt),
		UpdatedAt:  tsTime(r.UpdatedAt),
		Author: authorProfileFromThreadRow(
			r.AuthorID2, r.AuthorUsername, r.AuthorDisplayName, r.AuthorAvatarUrl,
			r.AuthorBio, r.AuthorWebsite, r.AuthorIsAdmin, r.AuthorForumlineID,
			r.AuthorCreatedAt, r.AuthorUpdatedAt,
		),
		Category: categoryFromThreadRow(
			r.CatID, r.CatName, r.CatSlug, r.CatDescription, r.CatSortOrder, r.CatCreatedAt,
		),
	}
}

// listUserThreadsRowToOapi converts a ListUserThreadsRow to oapi.Thread.
func listUserThreadsRowToOapi(r sqlcdb.ListUserThreadsRow) oapi.Thread {
	return oapi.Thread{
		Id:         pgUUID2OapiUUID(r.ID),
		CategoryId: pgUUID2OapiUUID(r.CategoryID),
		AuthorId:   pgUUID2OapiUUID(r.AuthorID),
		Title:      r.Title,
		Slug:       r.Slug,
		Content:    pgtextPtr(r.Content),
		ImageUrl:   pgtextPtr(r.ImageUrl),
		IsPinned:   r.IsPinned,
		IsLocked:   r.IsLocked,
		ViewCount:  int(r.ViewCount),
		PostCount:  int(r.PostCount),
		LastPostAt: tsTimePtr(r.LastPostAt),
		CreatedAt:  tsTime(r.CreatedAt),
		UpdatedAt:  tsTime(r.UpdatedAt),
		Author: authorProfileFromThreadRow(
			r.AuthorID2, r.AuthorUsername, r.AuthorDisplayName, r.AuthorAvatarUrl,
			r.AuthorBio, r.AuthorWebsite, r.AuthorIsAdmin, r.AuthorForumlineID,
			r.AuthorCreatedAt, r.AuthorUpdatedAt,
		),
		Category: categoryFromThreadRow(
			r.CatID, r.CatName, r.CatSlug, r.CatDescription, r.CatSortOrder, r.CatCreatedAt,
		),
	}
}

// searchThreadsRowToOapi converts a SearchThreadsRow to oapi.Thread.
func searchThreadsRowToOapi(r sqlcdb.SearchThreadsRow) oapi.Thread {
	return oapi.Thread{
		Id:         pgUUID2OapiUUID(r.ID),
		CategoryId: pgUUID2OapiUUID(r.CategoryID),
		AuthorId:   pgUUID2OapiUUID(r.AuthorID),
		Title:      r.Title,
		Slug:       r.Slug,
		Content:    pgtextPtr(r.Content),
		ImageUrl:   pgtextPtr(r.ImageUrl),
		IsPinned:   r.IsPinned,
		IsLocked:   r.IsLocked,
		ViewCount:  int(r.ViewCount),
		PostCount:  int(r.PostCount),
		LastPostAt: tsTimePtr(r.LastPostAt),
		CreatedAt:  tsTime(r.CreatedAt),
		UpdatedAt:  tsTime(r.UpdatedAt),
		Author: authorProfileFromThreadRow(
			r.AuthorID2, r.AuthorUsername, r.AuthorDisplayName, r.AuthorAvatarUrl,
			r.AuthorBio, r.AuthorWebsite, r.AuthorIsAdmin, r.AuthorForumlineID,
			r.AuthorCreatedAt, r.AuthorUpdatedAt,
		),
		Category: categoryFromThreadRow(
			r.CatID, r.CatName, r.CatSlug, r.CatDescription, r.CatSortOrder, r.CatCreatedAt,
		),
	}
}

// postRowToOapi converts a ListPostsByThreadRow to oapi.Post.
func postRowToOapi(r sqlcdb.ListPostsByThreadRow) oapi.Post {
	return oapi.Post{
		Id:        pgUUID2OapiUUID(r.ID),
		ThreadId:  pgUUID2OapiUUID(r.ThreadID),
		AuthorId:  pgUUID2OapiUUID(r.AuthorID),
		Content:   r.Content,
		ReplyToId: uuidPtrToOapi(r.ReplyToID),
		CreatedAt: tsTime(r.CreatedAt),
		UpdatedAt: tsTime(r.UpdatedAt),
		Author: authorProfileFromThreadRow(
			r.AuthorID2, r.AuthorUsername, r.AuthorDisplayName, r.AuthorAvatarUrl,
			r.AuthorBio, r.AuthorWebsite, r.AuthorIsAdmin, r.AuthorForumlineID,
			r.AuthorCreatedAt, r.AuthorUpdatedAt,
		),
	}
}

// listUserPostsRowToOapi converts a ListUserPostsRow to oapi.Post.
func listUserPostsRowToOapi(r sqlcdb.ListUserPostsRow) oapi.Post {
	return oapi.Post{
		Id:        pgUUID2OapiUUID(r.ID),
		ThreadId:  pgUUID2OapiUUID(r.ThreadID),
		AuthorId:  pgUUID2OapiUUID(r.AuthorID),
		Content:   r.Content,
		ReplyToId: uuidPtrToOapi(r.ReplyToID),
		CreatedAt: tsTime(r.CreatedAt),
		UpdatedAt: tsTime(r.UpdatedAt),
		Author: authorProfileFromThreadRow(
			r.AuthorID2, r.AuthorUsername, r.AuthorDisplayName, r.AuthorAvatarUrl,
			r.AuthorBio, r.AuthorWebsite, r.AuthorIsAdmin, r.AuthorForumlineID,
			r.AuthorCreatedAt, r.AuthorUpdatedAt,
		),
	}
}

// searchPostsRowToOapi converts a SearchPostsRow to oapi.Post.
func searchPostsRowToOapi(r sqlcdb.SearchPostsRow) oapi.Post {
	return oapi.Post{
		Id:        pgUUID2OapiUUID(r.ID),
		ThreadId:  pgUUID2OapiUUID(r.ThreadID),
		AuthorId:  pgUUID2OapiUUID(r.AuthorID),
		Content:   r.Content,
		ReplyToId: uuidPtrToOapi(r.ReplyToID),
		CreatedAt: tsTime(r.CreatedAt),
		UpdatedAt: tsTime(r.UpdatedAt),
		Author: authorProfileFromThreadRow(
			r.AuthorID2, r.AuthorUsername, r.AuthorDisplayName, r.AuthorAvatarUrl,
			r.AuthorBio, r.AuthorWebsite, r.AuthorIsAdmin, r.AuthorForumlineID,
			r.AuthorCreatedAt, r.AuthorUpdatedAt,
		),
	}
}

// bookmarkRowToOapi converts a ListBookmarksRow to oapi.Bookmark.
func bookmarkRowToOapi(r sqlcdb.ListBookmarksRow) oapi.Bookmark {
	return oapi.Bookmark{
		Id:        pgUUID2OapiUUID(r.ID),
		CreatedAt: tsTime(r.BookmarkCreatedAt),
		Thread: oapi.Thread{
			Id:         pgUUID2OapiUUID(r.ThreadID),
			CategoryId: pgUUID2OapiUUID(r.CategoryID),
			AuthorId:   pgUUID2OapiUUID(r.AuthorID),
			Title:      r.Title,
			Slug:       r.Slug,
			Content:    pgtextPtr(r.Content),
			ImageUrl:   pgtextPtr(r.ImageUrl),
			IsPinned:   r.IsPinned,
			IsLocked:   r.IsLocked,
			ViewCount:  int(r.ViewCount),
			PostCount:  int(r.PostCount),
			LastPostAt: tsTimePtr(r.LastPostAt),
			CreatedAt:  tsTime(r.ThreadCreatedAt),
			UpdatedAt:  tsTime(r.ThreadUpdatedAt),
			Author: authorProfileFromThreadRow(
				r.AuthorID2, r.AuthorUsername, r.AuthorDisplayName, r.AuthorAvatarUrl,
				r.AuthorBio, r.AuthorWebsite, r.AuthorIsAdmin, r.AuthorForumlineID,
				r.AuthorCreatedAt, r.AuthorUpdatedAt,
			),
			Category: categoryFromThreadRow(
				r.CatID, r.CatName, r.CatSlug, r.CatDescription, r.CatSortOrder, r.CatCreatedAt,
			),
		},
	}
}

// chatMessageRowToOapi converts a ListChatMessagesRow to oapi.ChatMessage.
func chatMessageRowToOapi(r sqlcdb.ListChatMessagesRow) oapi.ChatMessage {
	return oapi.ChatMessage{
		Id:        pgUUID2OapiUUID(r.ID),
		ChannelId: pgUUID2OapiUUID(r.ChannelID),
		AuthorId:  pgUUID2OapiUUID(r.AuthorID),
		Content:   r.Content,
		CreatedAt: tsTime(r.CreatedAt),
		Author: authorProfileFromThreadRow(
			r.AuthorID2, r.AuthorUsername, r.AuthorDisplayName, r.AuthorAvatarUrl,
			r.AuthorBio, r.AuthorWebsite, r.AuthorIsAdmin, r.AuthorForumlineID,
			r.AuthorCreatedAt, r.AuthorUpdatedAt,
		),
	}
}

// voicePresenceRowToOapi converts a ListVoicePresenceRow to oapi.VoicePresence.
func voicePresenceRowToOapi(r sqlcdb.ListVoicePresenceRow) oapi.VoicePresence {
	return oapi.VoicePresence{
		Id:       pgUUID2OapiUUID(r.ID),
		UserId:   pgUUID2OapiUUID(r.UserID),
		RoomSlug: r.RoomSlug,
		JoinedAt: tsTime(r.JoinedAt),
		Profile: authorProfileFromThreadRow(
			r.ProfileID, r.ProfileUsername, r.ProfileDisplayName, r.ProfileAvatarUrl,
			r.ProfileBio, r.ProfileWebsite, r.ProfileIsAdmin, r.ProfileForumlineID,
			r.ProfileCreatedAt, r.ProfileUpdatedAt,
		),
	}
}

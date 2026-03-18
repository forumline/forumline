package store

import (
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// profileFromSqlc converts a sqlcdb.Profile to an oapi.Profile.
// openapi_types.UUID is a type alias for uuid.UUID, so direct assignment works.
func profileFromSqlc(p sqlcdb.Profile) oapi.Profile {
	return oapi.Profile{
		Id:          p.ID,
		Username:    p.Username,
		DisplayName: p.DisplayName,
		AvatarUrl:   p.AvatarUrl,
		Bio:         p.Bio,
		Website:     p.Website,
		IsAdmin:     p.IsAdmin,
		ForumlineId: p.ForumlineID,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// authorProfileFromThreadRow extracts an oapi.Profile from thread row author fields.
func authorProfileFromThreadRow(
	id uuid.UUID, username string, displayName, avatarURL, bio, website *string,
	isAdmin bool, forumlineID *string, createdAt, updatedAt time.Time,
) oapi.Profile {
	return oapi.Profile{
		Id:          id,
		Username:    username,
		DisplayName: displayName,
		AvatarUrl:   avatarURL,
		Bio:         bio,
		Website:     website,
		IsAdmin:     isAdmin,
		ForumlineId: forumlineID,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

// categoryFromThreadRow extracts an oapi.Category from thread row category fields.
func categoryFromThreadRow(
	id uuid.UUID, name, slug string, description *string, sortOrder int32, createdAt time.Time,
) oapi.Category {
	return oapi.Category{
		Id:          id,
		Name:        name,
		Slug:        slug,
		Description: description,
		SortOrder:   int(sortOrder),
		CreatedAt:   createdAt,
	}
}

// --- Thread row conversion via common struct ---
//
// All 5 thread query row types (GetThreadRow, ListThreadsRow,
// ListThreadsByCategoryRow, ListUserThreadsRow, SearchThreadsRow)
// have identical struct layouts. We define threadFields with matching
// field names/types so that Go's struct conversion (e.g.,
// threadFields(row)) works without unsafe or reflection.

type threadFields struct {
	ID                uuid.UUID
	CategoryID        uuid.UUID
	AuthorID          uuid.UUID
	Title             string
	Slug              string
	Content           *string
	ImageUrl          *string
	IsPinned          bool
	IsLocked          bool
	ViewCount         int32
	PostCount         int32
	LastPostAt        time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	AuthorID2         uuid.UUID
	AuthorUsername    string
	AuthorDisplayName *string
	AuthorAvatarUrl   *string
	AuthorBio         *string
	AuthorWebsite     *string
	AuthorIsAdmin     bool
	AuthorForumlineID *string
	AuthorCreatedAt   time.Time
	AuthorUpdatedAt   time.Time
	CatID             uuid.UUID
	CatName           string
	CatSlug           string
	CatDescription    *string
	CatSortOrder      int32
	CatCreatedAt      time.Time
}

func threadToOapi(r threadFields) oapi.Thread {
	return oapi.Thread{
		Id:         r.ID,
		CategoryId: r.CategoryID,
		AuthorId:   r.AuthorID,
		Title:      r.Title,
		Slug:       r.Slug,
		Content:    r.Content,
		ImageUrl:   r.ImageUrl,
		IsPinned:   r.IsPinned,
		IsLocked:   r.IsLocked,
		ViewCount:  int(r.ViewCount),
		PostCount:  int(r.PostCount),
		LastPostAt: &r.LastPostAt,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
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

// Thin wrappers that do the struct conversion for each sqlcdb row type.

func threadRowToOapi(r sqlcdb.GetThreadRow) oapi.Thread {
	return threadToOapi(threadFields(r))
}

func listThreadsRowToOapi(r sqlcdb.ListThreadsRow) oapi.Thread {
	return threadToOapi(threadFields(r))
}

func listThreadsByCategoryRowToOapi(r sqlcdb.ListThreadsByCategoryRow) oapi.Thread {
	return threadToOapi(threadFields(r))
}

func listUserThreadsRowToOapi(r sqlcdb.ListUserThreadsRow) oapi.Thread {
	return threadToOapi(threadFields(r))
}

func searchThreadsRowToOapi(r sqlcdb.SearchThreadsRow) oapi.Thread {
	return threadToOapi(threadFields(r))
}

// --- Post row conversion via common struct ---
//
// All 3 post query row types (ListPostsByThreadRow, ListUserPostsRow,
// SearchPostsRow) have identical struct layouts.

type postFields struct {
	ID                uuid.UUID
	ThreadID          uuid.UUID
	AuthorID          uuid.UUID
	Content           string
	ReplyToID         *uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
	AuthorID2         uuid.UUID
	AuthorUsername    string
	AuthorDisplayName *string
	AuthorAvatarUrl   *string
	AuthorBio         *string
	AuthorWebsite     *string
	AuthorIsAdmin     bool
	AuthorForumlineID *string
	AuthorCreatedAt   time.Time
	AuthorUpdatedAt   time.Time
}

func postToOapi(r postFields) oapi.Post {
	var replyToID *openapi_types.UUID
	if r.ReplyToID != nil {
		v := *r.ReplyToID
		replyToID = &v
	}
	return oapi.Post{
		Id:        r.ID,
		ThreadId:  r.ThreadID,
		AuthorId:  r.AuthorID,
		Content:   r.Content,
		ReplyToId: replyToID,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
		Author: authorProfileFromThreadRow(
			r.AuthorID2, r.AuthorUsername, r.AuthorDisplayName, r.AuthorAvatarUrl,
			r.AuthorBio, r.AuthorWebsite, r.AuthorIsAdmin, r.AuthorForumlineID,
			r.AuthorCreatedAt, r.AuthorUpdatedAt,
		),
	}
}

func postRowToOapi(r sqlcdb.ListPostsByThreadRow) oapi.Post {
	return postToOapi(postFields(r))
}

func listUserPostsRowToOapi(r sqlcdb.ListUserPostsRow) oapi.Post {
	return postToOapi(postFields(r))
}

func searchPostsRowToOapi(r sqlcdb.SearchPostsRow) oapi.Post {
	return postToOapi(postFields(r))
}

// bookmarkRowToOapi converts a ListBookmarksRow to oapi.Bookmark.
// This has a unique structure (thread + bookmark fields), so it stays as-is.
func bookmarkRowToOapi(r sqlcdb.ListBookmarksRow) oapi.Bookmark {
	return oapi.Bookmark{
		Id:        r.ID,
		CreatedAt: r.BookmarkCreatedAt,
		Thread: oapi.Thread{
			Id:         r.ThreadID,
			CategoryId: r.CategoryID,
			AuthorId:   r.AuthorID,
			Title:      r.Title,
			Slug:       r.Slug,
			Content:    r.Content,
			ImageUrl:   r.ImageUrl,
			IsPinned:   r.IsPinned,
			IsLocked:   r.IsLocked,
			ViewCount:  int(r.ViewCount),
			PostCount:  int(r.PostCount),
			LastPostAt: &r.LastPostAt,
			CreatedAt:  r.ThreadCreatedAt,
			UpdatedAt:  r.ThreadUpdatedAt,
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
		Id:        r.ID,
		ChannelId: r.ChannelID,
		AuthorId:  r.AuthorID,
		Content:   r.Content,
		CreatedAt: r.CreatedAt,
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
		Id:       r.ID,
		UserId:   r.UserID,
		RoomSlug: r.RoomSlug,
		JoinedAt: r.JoinedAt,
		Profile: authorProfileFromThreadRow(
			r.ProfileID, r.ProfileUsername, r.ProfileDisplayName, r.ProfileAvatarUrl,
			r.ProfileBio, r.ProfileWebsite, r.ProfileIsAdmin, r.ProfileForumlineID,
			r.ProfileCreatedAt, r.ProfileUpdatedAt,
		),
	}
}

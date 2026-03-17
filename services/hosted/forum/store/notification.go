package store

import (
	"context"

	"github.com/forumline/forumline/services/hosted/forum/model"
	"github.com/forumline/forumline/services/hosted/sqlcdb"
)

// ListNotifications returns notifications for a user (data provider format).
func (s *Store) ListNotifications(ctx context.Context, userID string, limit int) ([]model.Notification, error) {
	rows, err := s.Q.ListNotifications(ctx, sqlcdb.ListNotificationsParams{
		UserID: pgUUID(userID),
		Limit:  int32(min(limit, 1000)),  //nolint:gosec // limit is bounded
	})
	if err != nil {
		return nil, err
	}
	notifications := make([]model.Notification, 0, len(rows))
	for _, r := range rows {
		notifications = append(notifications, model.Notification{
			ID:        uuidStr(r.ID),
			UserID:    uuidStr(r.UserID),
			Type:      r.Type,
			Title:     r.Title,
			Message:   r.Message,
			Link:      pgtextPtr(r.Link),
			Read:      r.Read,
			CreatedAt: tsStr(r.CreatedAt),
		})
	}
	return notifications, nil
}

// ListForumlineNotifications returns notifications in the forumline protocol format.
func (s *Store) ListForumlineNotifications(ctx context.Context, userID string, limit int, forumDomain string) ([]model.ForumlineNotification, error) {
	rows, err := s.Q.ListForumlineNotifications(ctx, sqlcdb.ListForumlineNotificationsParams{
		UserID: pgUUID(userID),
		Limit:  int32(min(limit, 1000)),  //nolint:gosec // limit is bounded
	})
	if err != nil {
		return nil, err
	}
	notifications := make([]model.ForumlineNotification, 0, len(rows))
	for _, r := range rows {
		link := "/"
		if r.Link.Valid {
			link = r.Link.String
		}
		notifications = append(notifications, model.ForumlineNotification{
			ID:          uuidStr(r.ID),
			Type:        r.Type,
			Title:       r.Title,
			Body:        r.Message,
			Link:        link,
			Read:        r.Read,
			Timestamp:   tsStr(r.CreatedAt),
			ForumDomain: forumDomain,
		})
	}
	return notifications, nil
}

// MarkNotificationRead marks a single notification as read.
func (s *Store) MarkNotificationRead(ctx context.Context, notifID, userID string) error {
	return s.Q.MarkNotificationRead(ctx, sqlcdb.MarkNotificationReadParams{
		ID:     pgUUID(notifID),
		UserID: pgUUID(userID),
	})
}

// MarkAllNotificationsRead marks all notifications as read for a user.
func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID string) error {
	return s.Q.MarkAllNotificationsRead(ctx, pgUUID(userID))
}

// InsertNotification inserts a notification.
func (s *Store) InsertNotification(ctx context.Context, userID, notifType, title, message, link string) error {
	return s.Q.InsertNotification(ctx, sqlcdb.InsertNotificationParams{
		UserID:  pgUUID(userID),
		Type:    notifType,
		Title:   title,
		Message: message,
		Link:    textToPgtext(link),
	})
}

// CountUnread returns unread notification and chat_mention counts.
func (s *Store) CountUnread(ctx context.Context, userID string) (notifs int, chatMentions int, err error) {
	uid := pgUUID(userID)
	n, err := s.Q.CountUnreadNotifications(ctx, uid)
	if err != nil {
		return 0, 0, err
	}
	c, err := s.Q.CountUnreadChatMentions(ctx, uid)
	if err != nil {
		return 0, 0, err
	}
	return int(n), int(c), nil
}

// GetThreadTitleAndAuthor returns the thread title and OP author ID.
func (s *Store) GetThreadTitleAndAuthor(ctx context.Context, threadID string) (title string, authorID string, err error) {
	row, err := s.Q.GetThreadTitleAndAuthor(ctx, pgUUID(threadID))
	if err != nil {
		return "", "", err
	}
	return row.Title, uuidStr(row.AuthorID), nil
}

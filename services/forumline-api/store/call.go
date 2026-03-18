package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forumline/forumline/services/forumline-api/model"
	"github.com/forumline/forumline/services/forumline-api/sqlcdb"
)

func (s *Store) GetCalleeFor1to1(ctx context.Context, userID string, conversationID uuid.UUID) (string, error) {
	return s.Q.GetCalleeFor1to1(ctx, sqlcdb.GetCalleeFor1to1Params{
		UserID:         userID,
		ConversationID: conversationID,
	})
}

func (s *Store) HasActiveCall(ctx context.Context, conversationID uuid.UUID) (bool, error) {
	return s.Q.HasActiveCall(ctx, conversationID)
}

func (s *Store) IsUserInCall(ctx context.Context, userID string) (bool, error) {
	return s.Q.IsUserInCall(ctx, userID)
}

func (s *Store) CreateCall(ctx context.Context, conversationID uuid.UUID, callerID, calleeID string) (*model.CallRecord, error) {
	row, err := s.Q.CreateCall(ctx, sqlcdb.CreateCallParams{
		ConversationID: conversationID,
		CallerID:       callerID,
		CalleeID:       calleeID,
	})
	if err != nil {
		return nil, err
	}
	return &model.CallRecord{
		ID:             row.ID,
		ConversationID: row.ConversationID,
		CallerID:       row.CallerID,
		CalleeID:       row.CalleeID,
		Status:         row.Status,
		CreatedAt:      row.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Store) GetRingingCallCallerID(ctx context.Context, callID uuid.UUID, calleeID string) (string, error) {
	return s.Q.GetRingingCallCallerID(ctx, sqlcdb.GetRingingCallCallerIDParams{
		ID:       callID,
		CalleeID: calleeID,
	})
}

func (s *Store) AcceptCall(ctx context.Context, callID uuid.UUID) error {
	return s.Q.AcceptCall(ctx, callID)
}

func (s *Store) DeclineCall(ctx context.Context, callID uuid.UUID) error {
	return s.Q.DeclineCall(ctx, callID)
}

func (s *Store) EndCall(ctx context.Context, callID uuid.UUID, userID string) (newStatus string, otherUserID string, err error) {
	row, err := s.Q.GetCallForEnd(ctx, sqlcdb.GetCallForEndParams{
		CallID: callID,
		UserID: userID,
	})
	if err != nil {
		return "", "", err
	}

	newStatus = "completed"
	if row.Status == "ringing" {
		if userID == row.CallerID {
			newStatus = "cancelled"
		} else {
			newStatus = "missed"
		}
	}

	if row.Status == "active" && row.StartedAt != nil {
		err = s.Q.EndCallWithDuration(ctx, sqlcdb.EndCallWithDurationParams{
			Status: newStatus,
			ID:     callID,
		})
	} else {
		err = s.Q.EndCallWithoutDuration(ctx, sqlcdb.EndCallWithoutDurationParams{
			Status: newStatus,
			ID:     callID,
		})
	}
	if err != nil {
		return "", "", err
	}

	otherUserID = row.CalleeID
	if userID == row.CalleeID {
		otherUserID = row.CallerID
	}
	return newStatus, otherUserID, nil
}

func (s *Store) IsCallParticipant(ctx context.Context, callID uuid.UUID, userID string) (bool, error) {
	return s.Q.IsCallParticipant(ctx, sqlcdb.IsCallParticipantParams{
		ID:       callID,
		CallerID: userID,
	})
}

func (s *Store) CleanupStaleCalls(ctx context.Context) (int64, error) {
	return s.Q.CleanupStaleCalls(ctx)
}

func (s *Store) NotifyCallSignal(ctx context.Context, payload string) error {
	return s.Q.NotifyCallSignal(ctx, payload)
}

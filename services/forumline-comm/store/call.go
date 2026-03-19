package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forumline/forumline/services/forumline-comm/sqlcdb"
)

type CallRecord struct {
	ID              uuid.UUID `json:"id"`
	ConversationID  uuid.UUID `json:"conversation_id"`
	CallerID        string    `json:"caller_id"`
	CalleeID        string    `json:"callee_id"`
	Status          string    `json:"status"`
	RoomName        string    `json:"room_name,omitempty"`
	CreatedAt       string    `json:"created_at"`
	StartedAt       *string   `json:"started_at,omitempty"`
	EndedAt         *string   `json:"ended_at,omitempty"`
	DurationSeconds *int      `json:"duration_seconds,omitempty"`
}

func (s *Store) GetCalleeFor1to1(ctx context.Context, userID string, conversationID uuid.UUID) (string, error) {
	return s.Q.GetCalleeFor1to1(ctx, sqlcdb.GetCalleeFor1to1Params{
		UserID:         userID,
		ConversationID: conversationID,
	})
}

func (s *Store) CreateCallRecord(ctx context.Context, conversationID uuid.UUID, callerID, calleeID, roomName string) (*CallRecord, error) {
	row, err := s.Q.CreateCallRecord(ctx, sqlcdb.CreateCallRecordParams{
		ConversationID: conversationID,
		CallerID:       callerID,
		CalleeID:       calleeID,
		RoomName:       &roomName,
	})
	if err != nil {
		return nil, err
	}
	rn := ""
	if row.RoomName != nil {
		rn = *row.RoomName
	}
	return &CallRecord{
		ID:             row.ID,
		ConversationID: row.ConversationID,
		CallerID:       row.CallerID,
		CalleeID:       row.CalleeID,
		Status:         row.Status,
		RoomName:       rn,
		CreatedAt:      row.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Store) GetCallByID(ctx context.Context, callID uuid.UUID) (*CallRecord, error) {
	row, err := s.Q.GetCallByID(ctx, callID)
	if err != nil {
		return nil, err
	}
	return callRowToRecord(row.ID, row.ConversationID, row.CallerID, row.CalleeID,
		row.Status, row.RoomName, row.CreatedAt, row.StartedAt, row.EndedAt, row.DurationSeconds), nil
}

func (s *Store) GetCallByRoomName(ctx context.Context, roomName string) (*CallRecord, error) {
	row, err := s.Q.GetCallByRoomName(ctx, &roomName)
	if err != nil {
		return nil, err
	}
	return callRowToRecord(row.ID, row.ConversationID, row.CallerID, row.CalleeID,
		row.Status, row.RoomName, row.CreatedAt, row.StartedAt, row.EndedAt, row.DurationSeconds), nil
}

func (s *Store) UpdateCallStatus(ctx context.Context, callID uuid.UUID, status string) error {
	return s.Q.UpdateCallStatus(ctx, sqlcdb.UpdateCallStatusParams{
		Status: status,
		ID:     callID,
	})
}

func (s *Store) ActivateCall(ctx context.Context, callID uuid.UUID) error {
	return s.Q.ActivateCall(ctx, callID)
}

func (s *Store) EndCallWithDuration(ctx context.Context, callID uuid.UUID, status string, durationSec int) error {
	return s.Q.EndCallWithDuration(ctx, sqlcdb.EndCallWithDurationParams{
		Status:          status,
		DurationSeconds: pgtype.Int4{Int32: int32(durationSec), Valid: true},
		ID:              callID,
	})
}

func (s *Store) EndCallWithoutDuration(ctx context.Context, callID uuid.UUID, status string) error {
	return s.Q.EndCallWithoutDuration(ctx, sqlcdb.EndCallWithoutDurationParams{
		Status: status,
		ID:     callID,
	})
}

func (s *Store) IsCallParticipant(ctx context.Context, callID uuid.UUID, userID string) (bool, error) {
	return s.Q.IsCallParticipant(ctx, sqlcdb.IsCallParticipantParams{
		ID:       callID,
		CallerID: userID,
	})
}

func callRowToRecord(
	id, conversationID uuid.UUID,
	callerID, calleeID, status string,
	roomName *string,
	createdAt time.Time,
	startedAt, endedAt *time.Time,
	durationSeconds pgtype.Int4,
) *CallRecord {
	rec := &CallRecord{
		ID:             id,
		ConversationID: conversationID,
		CallerID:       callerID,
		CalleeID:       calleeID,
		Status:         status,
		CreatedAt:      createdAt.Format(time.RFC3339),
	}
	if roomName != nil {
		rec.RoomName = *roomName
	}
	if startedAt != nil {
		t := startedAt.Format(time.RFC3339)
		rec.StartedAt = &t
	}
	if endedAt != nil {
		t := endedAt.Format(time.RFC3339)
		rec.EndedAt = &t
	}
	if durationSeconds.Valid {
		d := int(durationSeconds.Int32)
		rec.DurationSeconds = &d
	}
	return rec
}

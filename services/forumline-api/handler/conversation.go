package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"

	"github.com/forumline/forumline/backend/auth"
)

type ConversationHandler struct {
	Hasura *HasuraClient
}

func NewConversationHandler(hasura *HasuraClient) *ConversationHandler {
	return &ConversationHandler{Hasura: hasura}
}

func (h *ConversationHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	uid, _ := uuid.Parse(userID)
	convos, err := h.Hasura.ListConversations(r.Context(), uid)
	if err != nil {
		log.Printf("[Conv] list error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch"})
		return
	}
	writeJSON(w, http.StatusOK, convos)
}

func (h *ConversationHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	uid, _ := uuid.Parse(userID)
	convoID, _ := uuid.Parse(r.PathValue("conversationId"))
	c, err := h.Hasura.GetConversation(r.Context(), uid, convoID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *ConversationHandler) HandleGetOrCreateDM(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	uid, _ := uuid.Parse(userID)
	var body struct {
		UserID string `json:"userId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	otherID, _ := uuid.Parse(body.UserID)
	convoID, err := h.Hasura.FindOrCreate1to1(r.Context(), uid, otherID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": convoID.String()})
}

func (h *ConversationHandler) HandleCreateGroup(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	uid, _ := uuid.Parse(userID)
	var body struct {
		MemberIDs []string `json:"memberIds"`
		Name      string   `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id, err := h.Hasura.CreateGroupConversation(r.Context(), uid, body.Name, body.MemberIDs)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (h *ConversationHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "Update via Hasura mutations"})
}

func (h *ConversationHandler) HandleGetMessages(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	uid, _ := uuid.Parse(userID)
	convoID, _ := uuid.Parse(r.PathValue("conversationId"))
	msgs, err := h.Hasura.GetMessages(r.Context(), uid, convoID, nil, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed"})
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (h *ConversationHandler) HandleSendMessage(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	uid, _ := uuid.Parse(userID)
	convoID, _ := uuid.Parse(r.PathValue("conversationId"))
	var body struct {
		Content string `json:"content"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	msg, err := h.Hasura.SendMessage(r.Context(), convoID, uid, body.Content)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, msg)
}

func (h *ConversationHandler) HandleMarkRead(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	uid, _ := uuid.Parse(userID)
	convoID, _ := uuid.Parse(r.PathValue("conversationId"))
	_ = h.Hasura.MarkRead(r.Context(), convoID, uid)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *ConversationHandler) HandleLeave(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	uid, _ := uuid.Parse(userID)
	convoID, _ := uuid.Parse(r.PathValue("conversationId"))
	if err := h.Hasura.LeaveConversation(r.Context(), convoID, uid); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// Legacy handlers delegate to new ones
func (h *ConversationHandler) HandleLegacyGetMessages(w http.ResponseWriter, r *http.Request) {
	r.SetPathValue("conversationId", r.PathValue("userId")) // hack for legacy routes
	h.HandleGetMessages(w, r)
}

func (h *ConversationHandler) HandleLegacySendMessage(w http.ResponseWriter, r *http.Request) {
	r.SetPathValue("conversationId", r.PathValue("userId"))
	h.HandleSendMessage(w, r)
}

func (h *ConversationHandler) HandleLegacyMarkRead(w http.ResponseWriter, r *http.Request) {
	r.SetPathValue("conversationId", r.PathValue("userId"))
	h.HandleMarkRead(w, r)
}

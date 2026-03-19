package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type HasuraClient struct {
	endpoint   string
	adminToken string
	client     *http.Client
}

func NewHasuraClient(endpoint, adminToken string) *HasuraClient {
	return &HasuraClient{
		endpoint:   strings.TrimSuffix(endpoint, "/"),
		adminToken: adminToken,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

type GraphQLRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

type GraphQLResponse struct {
	Data   interface{}       `json:"data"`
	Errors []GraphQLError    `json:"errors"`
	Extensions interface{} `json:"extensions,omitempty"`
}

type GraphQLError struct {
	Message    string                 `json:"message"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

func (h *HasuraClient) query(ctx context.Context, q string, vars map[string]interface{}) (map[string]interface{}, error) {
	req := GraphQLRequest{Query: q, Variables: vars}
	body, _ := json.Marshal(req)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", h.endpoint+"/v1/graphql", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Hasura-Admin-Secret", h.adminToken)

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("hasura request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	var gqlResp GraphQLResponse
	_ = json.Unmarshal(respBody, &gqlResp)

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
	}

	if data, ok := gqlResp.Data.(map[string]interface{}); ok {
		return data, nil
	}
	return nil, fmt.Errorf("invalid response format")
}

// ListConversations - Get all conversations for a user with unread counts
func (h *HasuraClient) ListConversations(ctx context.Context, userID uuid.UUID) ([]map[string]interface{}, error) {
	q := `
		query ListConversations($userID: uuid!) {
			forumline_conversations(
				order_by: {updated_at: desc}
				where: {members: {user_id: {_eq: $userID}}}
			) {
				id
				is_group
				name
				created_at
				updated_at
				created_by
				members(where: {}) {
					user_id
					joined_at
					last_read_at
					forumline_profiles {
						id
						display_name
						avatar_color
					}
				}
				direct_messages(limit: 1, order_by: {created_at: desc}) {
					id
					sender_id
					content
					created_at
					forumline_profiles {
						display_name
					}
				}
			}
		}
	`

	data, err := h.query(ctx, q, map[string]interface{}{"userID": userID})
	if err != nil {
		return nil, err
	}

	convos, ok := data["forumline_conversations"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	result := make([]map[string]interface{}, len(convos))
	for i, c := range convos {
		if m, ok := c.(map[string]interface{}); ok {
			result[i] = m
		}
	}
	return result, nil
}

// GetConversation - Get single conversation with members
func (h *HasuraClient) GetConversation(ctx context.Context, userID, conversationID uuid.UUID) (map[string]interface{}, error) {
	q := `
		query GetConversation($id: uuid!, $userID: uuid!) {
			forumline_conversations_by_pk(id: $id) {
				id
				is_group
				name
				created_at
				created_by
				members(where: {user_id: {_eq: $userID}}) {
					user_id
				}
				members {
					user_id
					joined_at
					forumline_profiles {
						id
						display_name
						avatar_color
					}
				}
			}
		}
	`

	data, err := h.query(ctx, q, map[string]interface{}{"id": conversationID, "userID": userID})
	if err != nil {
		return nil, err
	}

	convo, ok := data["forumline_conversations_by_pk"].(map[string]interface{})
	if !ok || convo == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	// Check membership
	members, ok := convo["members"].([]interface{})
	if !ok || len(members) == 0 {
		return nil, fmt.Errorf("not a member of this conversation")
	}

	return convo, nil
}

// FindOrCreate1to1 - Transactional find-or-create for DM
func (h *HasuraClient) FindOrCreate1to1(ctx context.Context, userID, otherUserID uuid.UUID) (uuid.UUID, error) {
	// First, try to find existing 1:1
	q := `
		query Find1to1($user1: uuid!, $user2: uuid!) {
			forumline_conversations(
				where: {
					is_group: {_eq: false}
					members: {
						user_id: {_in: [$user1, $user2]}
					}
				}
			) {
				id
				members_aggregate {
					aggregate {
						count
					}
				}
			}
		}
	`

	data, err := h.query(ctx, q, map[string]interface{}{
		"user1": userID,
		"user2": otherUserID,
	})
	if err != nil {
		log.Printf("[Hasura] find 1:1 error: %v", err)
	} else if data != nil {
		convos, ok := data["forumline_conversations"].([]interface{})
		if ok && len(convos) > 0 {
			for _, c := range convos {
				if conv, ok := c.(map[string]interface{}); ok {
					if agg, ok := conv["members_aggregate"].(map[string]interface{}); ok {
						if aggregate, ok := agg["aggregate"].(map[string]interface{}); ok {
							if count, ok := aggregate["count"].(float64); ok && count == 2 {
								if id, ok := conv["id"].(string); ok {
									if parsed, err := uuid.Parse(id); err == nil {
										return parsed, nil
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Create new 1:1
	createQ := `
		mutation CreateDM($user1: uuid!, $user2: uuid!) {
			insert_forumline_conversations_one(
				object: {
					is_group: false
					created_by: $user1
					members: {
						data: [
							{user_id: $user1}
							{user_id: $user2}
						]
					}
				}
			) {
				id
			}
		}
	`

	createData, err := h.query(ctx, createQ, map[string]interface{}{
		"user1": userID,
		"user2": otherUserID,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create 1:1 failed: %w", err)
	}

	result, ok := createData["insert_forumline_conversations_one"].(map[string]interface{})
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid create response")
	}

	idStr, ok := result["id"].(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("no id in response")
	}

	return uuid.Parse(idStr)
}

// CreateGroupConversation - Create group with members and validation
func (h *HasuraClient) CreateGroupConversation(ctx context.Context, userID uuid.UUID, name string, memberIDs []string) (uuid.UUID, error) {
	// Validate members exist
	if len(memberIDs) < 2 {
		return uuid.Nil, fmt.Errorf("group needs at least 2 members + creator")
	}
	if len(memberIDs) > 50 {
		return uuid.Nil, fmt.Errorf("group limited to 50 members")
	}

	checkQ := `
		query CheckUsersExist($ids: [uuid!]!) {
			forumline_profiles_aggregate(where: {id: {_in: $ids}}) {
				aggregate {
					count
				}
			}
		}
	`

	checkData, err := h.query(ctx, checkQ, map[string]interface{}{"ids": memberIDs})
	if err != nil {
		return uuid.Nil, fmt.Errorf("member validation failed: %w", err)
	}

	agg, ok := checkData["forumline_profiles_aggregate"].(map[string]interface{})
	if !ok {
		return uuid.Nil, fmt.Errorf("unexpected response")
	}

	aggData, ok := agg["aggregate"].(map[string]interface{})
	if !ok {
		return uuid.Nil, fmt.Errorf("unexpected response format")
	}

	count, ok := aggData["count"].(float64)
	if !ok || count != float64(len(memberIDs)) {
		return uuid.Nil, fmt.Errorf("some members don't exist")
	}

	// Build members list (include creator)
	members := []map[string]string{{"user_id": userID.String()}}
	for _, id := range memberIDs {
		if id != userID.String() {
			members = append(members, map[string]string{"user_id": id})
		}
	}

	createQ := `
		mutation CreateGroup($name: String!, $creatorID: uuid!, $members: [forumline_conversation_members_insert_input!]!) {
			insert_forumline_conversations_one(
				object: {
					is_group: true
					name: $name
					created_by: $creatorID
					members: {data: $members}
				}
			) {
				id
			}
		}
	`

	createData, err := h.query(ctx, createQ, map[string]interface{}{
		"name":      name,
		"creatorID": userID,
		"members":   members,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create group failed: %w", err)
	}

	result, ok := createData["insert_forumline_conversations_one"].(map[string]interface{})
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid response")
	}

	idStr, ok := result["id"].(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("no id in response")
	}

	return uuid.Parse(idStr)
}

// GetMessages - Paginated messages with cursor
func (h *HasuraClient) GetMessages(ctx context.Context, userID, conversationID uuid.UUID, before *time.Time, limit int) ([]map[string]interface{}, error) {
	if limit > 100 {
		limit = 100
	}

	q := `
		query GetMessages($convoID: uuid!, $userID: uuid!, $before: timestamptz, $limit: Int!) {
			forumline_direct_messages(
				where: {
					conversation_id: {_eq: $convoID}
					created_at: {_lt: $before}
				}
				order_by: {created_at: desc}
				limit: $limit
			) {
				id
				sender_id
				content
				created_at
				forumline_profiles {
					id
					display_name
					avatar_color
				}
			}
		}
	`

	vars := map[string]interface{}{
		"convoID": conversationID,
		"userID":  userID,
		"limit":   limit,
	}
	if before != nil {
		vars["before"] = *before
	}

	data, err := h.query(ctx, q, vars)
	if err != nil {
		return nil, err
	}

	messages, ok := data["forumline_direct_messages"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response")
	}

	result := make([]map[string]interface{}, len(messages))
	for i, m := range messages {
		if msg, ok := m.(map[string]interface{}); ok {
			result[i] = msg
		}
	}

	// Reverse to chronological order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

// SendMessage - Insert message and return it
func (h *HasuraClient) SendMessage(ctx context.Context, conversationID, senderID uuid.UUID, content string) (map[string]interface{}, error) {
	q := `
		mutation SendMessage($convoID: uuid!, $senderID: uuid!, $content: String!) {
			insert_forumline_direct_messages_one(
				object: {
					conversation_id: $convoID
					sender_id: $senderID
					content: $content
				}
			) {
				id
				sender_id
				content
				created_at
				forumline_profiles {
					display_name
				}
			}
		}
	`

	data, err := h.query(ctx, q, map[string]interface{}{
		"convoID":  conversationID,
		"senderID": senderID,
		"content":  content,
	})
	if err != nil {
		return nil, err
	}

	msg, ok := data["insert_forumline_direct_messages_one"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response")
	}

	return msg, nil
}

// MarkRead - Update last_read_at for user
func (h *HasuraClient) MarkRead(ctx context.Context, conversationID, userID uuid.UUID) error {
	q := `
		mutation MarkRead($convoID: uuid!, $userID: uuid!) {
			update_forumline_conversation_members(
				where: {conversation_id: {_eq: $convoID}, user_id: {_eq: $userID}}
				_set: {last_read_at: "now()"}
			) {
				affected_rows
			}
		}
	`

	_, err := h.query(ctx, q, map[string]interface{}{
		"convoID": conversationID,
		"userID":  userID,
	})
	return err
}

// LeaveConversation - Remove user from group conversation
func (h *HasuraClient) LeaveConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	// Check if group
	checkQ := `
		query CheckGroup($id: uuid!) {
			forumline_conversations_by_pk(id: $id) {
				is_group
			}
		}
	`

	data, _ := h.query(ctx, checkQ, map[string]interface{}{"id": conversationID})
	if data != nil {
		if convo, ok := data["forumline_conversations_by_pk"].(map[string]interface{}); ok {
			if isGroup, ok := convo["is_group"].(bool); ok && !isGroup {
				return fmt.Errorf("cannot leave 1:1 conversation")
			}
		}
	}

	q := `
		mutation Leave($convoID: uuid!, $userID: uuid!) {
			delete_forumline_conversation_members(
				where: {conversation_id: {_eq: $convoID}, user_id: {_eq: $userID}}
			) {
				affected_rows
			}
		}
	`

	_, err := h.query(ctx, q, map[string]interface{}{
		"convoID": conversationID,
		"userID":  userID,
	})
	return err
}

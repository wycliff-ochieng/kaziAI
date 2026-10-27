package handlers

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"erpagent-backend/internal/chathistory"
	"erpagent-backend/middleware"
)

func chatIdentity(r *http.Request) (tenantID, userID string, ok bool) {
	tenantID, _ = r.Context().Value(middleware.TenantIDKey).(string)
	userID, _ = r.Context().Value(middleware.UserIDKey).(string)
	if tenantID == "" || userID == "" || userID == "service" || userID == "dev" {
		return "", "", false
	}
	return tenantID, userID, true
}

// ListChats returns the signed-in user's conversations, newest first.
func ListChats(store chathistory.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, userID, ok := chatIdentity(r)
		if !ok {
			RespondError(w, http.StatusUnauthorized, "A signed-in account is required")
			return
		}
		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
				limit = parsed
			}
		}
		conversations, err := store.ListConversations(r.Context(), tenantID, userID, limit)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, "Could not load chat history")
			return
		}
		RespondJSON(w, http.StatusOK, map[string]any{"status": "success", "data": conversations})
	}
}

// GetChat returns one conversation and its messages.
func GetChat(store chathistory.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, userID, ok := chatIdentity(r)
		if !ok {
			RespondError(w, http.StatusUnauthorized, "A signed-in account is required")
			return
		}
		conversationID := mux.Vars(r)["id"]
		messages, err := store.GetMessages(r.Context(), tenantID, userID, conversationID)
		if err == chathistory.ErrNotFound {
			RespondError(w, http.StatusNotFound, "Conversation not found")
			return
		}
		if err != nil {
			RespondError(w, http.StatusInternalServerError, "Could not load conversation")
			return
		}
		RespondJSON(w, http.StatusOK, map[string]any{
			"status":          "success",
			"conversation_id": conversationID,
			"data":            messages,
		})
	}
}

// DeleteChat removes one conversation and its messages.
func DeleteChat(store chathistory.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, userID, ok := chatIdentity(r)
		if !ok {
			RespondError(w, http.StatusUnauthorized, "A signed-in account is required")
			return
		}
		conversationID := mux.Vars(r)["id"]
		if err := store.DeleteConversation(r.Context(), tenantID, userID, conversationID); err != nil {
			if err == chathistory.ErrNotFound {
				RespondError(w, http.StatusNotFound, "Conversation not found")
				return
			}
			RespondError(w, http.StatusInternalServerError, "Could not delete conversation")
			return
		}
		RespondJSON(w, http.StatusOK, map[string]string{"status": "success"})
	}
}

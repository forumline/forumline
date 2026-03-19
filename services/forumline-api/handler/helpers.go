package handler

import (
	"net/http"
	"strings"

	"github.com/forumline/forumline/backend/httpkit"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	httpkit.WriteJSON(w, status, v)
}

func trimString(s string) string {
	return strings.TrimSpace(s)
}

package handler

import (
	"net/http"

	"github.com/forumline/forumline/backend/httpkit"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	httpkit.WriteJSON(w, status, v)
}

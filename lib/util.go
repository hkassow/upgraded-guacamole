package lib

import (
	"net/http"
	"github.com/gorilla/sessions"
)

func GetUserID(r *http.Request, store *sessions.CookieStore) (int) {
	session, err := store.Get(r, "session")
	if err != nil {
		return 0
	}

	userIDRaw, ok := session.Values["user_id"]
	if !ok {
		return 0
	}

	switch v := userIDRaw.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

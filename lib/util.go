package lib

import (
	"net/http"
	"github.com/gorilla/sessions"
	"context"
	"go-guacamole/db"
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

func GetUserIdByUuid(ctx context.Context, uuid string) (int) {
    var id int
    err := db.Pool.QueryRow(ctx,
        `SELECT id FROM users WHERE uuid = $1`,
        uuid,
    ).Scan(&id)

	if err != nil {
        return 0
    }

	return id
}
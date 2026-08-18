package handlers

import (
    "encoding/json"
    "net/http"

	"go-guacamole/lib"
	"go-guacamole/db"

	"github.com/jackc/pgx/v5"
	"github.com/google/uuid"
)

type FollowUserRequest struct {
    FriendCode     string `json:"friend_code"`
}

func FollowNewUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	followerID := lib.GetUserID(r, store)

	var req FollowUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	_, parseErr := uuid.Parse(req.FriendCode)
	if parseErr != nil {
		http.Error(w, "invalid friend code", http.StatusBadRequest)
		return
	}

	var followeeID int

	err := db.Pool.QueryRow(ctx, `
		SELECT id FROM users WHERE uuid = $1
	`, req.FriendCode).Scan(&followeeID)

	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "user does not exist", http.StatusBadRequest)
			return
		}

		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	if followeeID == followerID {
		http.Error(w, "cannot follow yourself", http.StatusBadRequest)
		return
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO users_follows (follower_id, followee_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, followerID, followeeID)

	if err != nil {
		http.Error(w, "failed to follow user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"followed"}`))
}
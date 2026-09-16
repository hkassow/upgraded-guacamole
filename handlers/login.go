package handlers

import (
    "encoding/json"
    "net/http"
    "log"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"github.com/gorilla/sessions"
    "github.com/jackc/pgx/v5"

	"go-guacamole/db"
    "go-guacamole/lib"
)

var googleOauthConfig *oauth2.Config
var store *sessions.CookieStore

func InitGoogle() {
	internalKey, err := lib.LoadSecret("INTERNAL_API_KEY")
	if err != nil {
		log.Fatal(err)
	}

	store = sessions.NewCookieStore([]byte(internalKey))

	clientID, err := lib.LoadSecret("GOOGLE_CLIENT_ID")
	if err != nil {
		log.Fatal(err)
	}

	clientSecret, err := lib.LoadSecret("GOOGLE_CLIENT_SECRET")
	if err != nil {
		log.Fatal(err)
	}

	googleOauthConfig = &oauth2.Config{
		RedirectURL:  "https://upgraded-guacamole.com/auth/google/callback",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

func GoogleLogin(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "oauth-state")

	state := "random-state" // ideally replace with crypto/rand string
	session.Values["state"] = state
	session.Save(r, w)

	url := googleOauthConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func GoogleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
    pool := db.Pool

    // 1. Exchange OAuth code
	code := r.URL.Query().Get("code")

	token, err := googleOauthConfig.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "OAuth exchange failed", 500)
		return
    }

    // 2. Get user info from Google
	client := googleOauthConfig.Client(ctx, token)

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		http.Error(w, "Failed getting user info", 500)
		return
	}
	defer resp.Body.Close()

	var userInfo struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}

    if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		http.Error(w, "Failed parsing user info", 500)
		return
	}
    
    log.Println("Processing oauth:", userInfo.ID, userInfo.Email, userInfo.Name)

    // 3. Find or create user in DB
	var userID int

	err = pool.QueryRow(ctx,
		`SELECT id FROM users WHERE google_id = $1`,
		userInfo.ID,
	).Scan(&userID)

	if err != nil {
		if err == pgx.ErrNoRows {

			err = pool.QueryRow(ctx,
				`INSERT INTO users (google_id, email, display_name)
				 VALUES ($1, $2, $3)
				 RETURNING id`,
				userInfo.ID,
				userInfo.Email,
				userInfo.Name,
			).Scan(&userID)

			if err != nil {
				http.Error(w, "DB insert failed", 500)
				return
			}

		} else {
			http.Error(w, "DB query failed", 500)
			return
		}
	}

	
    // 4. Create session
    if err := CreateSession(w, r, userID); err != nil {
		http.Error(w, "Failed creating session", 500)
		return
	}

	// 5. Redirect to app
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func CreateSession(w http.ResponseWriter, r *http.Request, userID int) error {
    session, _ := store.Get(r, "session")

    session.Values["user_id"] = userID

    session.Options = &sessions.Options{
        Path:     "/",
        MaxAge:   86400 * 30,
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteLaxMode,
    }

    return session.Save(r, w)
}

func MeHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")

	userID, ok := session.Values["user_id"]
	if !ok {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	// fetch user from DB
	var user struct {
		Uuid 		string
		DisplayName string
		Email       string
	}


	err := db.Pool.QueryRow(r.Context(),
		`SELECT uuid, display_name, email FROM users WHERE id = $1`,
		userID,
	).Scan(&user.Uuid, &user.DisplayName, &user.Email)

	if err != nil {
		http.Error(w, "user not found", 404)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "session")
	if err != nil {
		http.Error(w, "Failed getting session", http.StatusInternalServerError)
		return
	}

	// Delete the session cookie
	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}

	// Remove the user from the session
	delete(session.Values, "user_id")

	if err := session.Save(r, w); err != nil {
		http.Error(w, "Failed logging out", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
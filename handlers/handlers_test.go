package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/gorilla/sessions"

	"go-guacamole/internal/testutil"
	"go-guacamole/lib"
	"go-guacamole/models"
)

func TestMain(m *testing.M) {
	// InitGoogle normally sets this up from secrets; tests only need a cookie store to sign sessions
	store = sessions.NewCookieStore([]byte("test-session-key-0123456789abcdef"))
	os.Exit(m.Run())
}

// request sends a request to handler, logged in as user if it isn't nil. body is JSON encoded
// unless it's already a string.
func request(t *testing.T, handler http.HandlerFunc, method, target string, body any, user *testutil.User) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	switch b := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case string:
		reader = bytes.NewReader([]byte(b))
	default:
		encoded, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}

	req := httptest.NewRequest(method, target, reader)
	if user != nil {
		req.AddCookie(sessionCookie(t, user.ID))
	}

	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

// sessionCookie returns a signed session cookie for userID, the same one GoogleCallback would set.
func sessionCookie(t *testing.T, userID int) *http.Cookie {
	t.Helper()

	rec := httptest.NewRecorder()
	if err := CreateSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), userID); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	t.Fatal("CreateSession did not set a session cookie")
	return nil
}

func expectStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, want, strings.TrimSpace(rec.Body.String()))
	}
}

func decodeRecipes(t *testing.T, rec *httptest.ResponseRecorder) []lib.RecipeResponse {
	t.Helper()

	var recipes []lib.RecipeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &recipes); err != nil {
		t.Fatalf("decoding recipes %q: %v", rec.Body.String(), err)
	}
	return recipes
}

func manualRecipe(name string) models.RawRecipe {
	return models.RawRecipe{
		Name: name,
		Type: "manual",
		Text: "Mix.\nBake.",
		Ingredients: []models.RawIngredient{
			{Name: "flour", Amount: "1 cup"},
			{Name: "sugar", Amount: "2 tbsp"},
		},
	}
}

// createManualRecipe posts a manual recipe as user and returns it as GET /recipes shows it.
func createManualRecipe(t *testing.T, user testutil.User, name string) lib.RecipeResponse {
	t.Helper()

	expectStatus(t, request(t, RecipesHandler, http.MethodPost, "/recipes", manualRecipe(name), &user), http.StatusCreated)
	return createdRecipe(t, user, name)
}

func drainRecipeQueue() []models.RecipeJob {
	var jobs []models.RecipeJob
	for {
		select {
		case job := <-lib.RecipeQueue:
			jobs = append(jobs, job)
		default:
			return jobs
		}
	}
}

// ---------------------------------------------------------------------------
// /recipes
// ---------------------------------------------------------------------------

func TestRecipesMethodNotAllowed(t *testing.T) {
	expectStatus(t, request(t, RecipesHandler, http.MethodPut, "/recipes", nil, nil), http.StatusMethodNotAllowed)
}

func TestPostRecipeValidation(t *testing.T) {
	someone := &testutil.User{ID: 1} // validation happens before any database access
	manual := manualRecipe("Cake")

	noIngredients := manual
	noIngredients.Ingredients = nil
	noInstructions := manual
	noInstructions.Text = ""

	tests := []struct {
		name string
		body any
		want string
	}{
		{"invalid json", "{", "Invalid JSON"},
		{"missing name", models.RawRecipe{Type: "text", Text: "..."}, "Recipe name is required"},
		{"text without text", models.RawRecipe{Name: "Cake", Type: "text"}, "Recipe text is required"},
		{"image without images", models.RawRecipe{Name: "Cake", Type: "image"}, "At least one recipe image is required"},
		{"manual without instructions", noInstructions, "Recipe instructions are required"},
		{"manual without ingredients", noIngredients, "At least one ingredient is required"},
		{"unknown type", models.RawRecipe{Name: "Cake", Type: "video", Text: "..."}, "Invalid recipe type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := request(t, RecipesHandler, http.MethodPost, "/recipes", tt.body, someone)
			expectStatus(t, rec, http.StatusBadRequest)
			if !strings.Contains(rec.Body.String(), tt.want) {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.want)
			}
		})
	}
}

func TestPostRecipeRequiresLogin(t *testing.T) {
	drainRecipeQueue()
	rec := request(t, RecipesHandler, http.MethodPost, "/recipes", models.RawRecipe{Name: "Cake", Type: "text", Text: "..."}, nil)
	expectStatus(t, rec, http.StatusUnauthorized)

	if jobs := drainRecipeQueue(); len(jobs) != 0 {
		t.Errorf("queued %d jobs for a logged out request", len(jobs))
	}
}

func TestPostManualRecipe(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	r := createManualRecipe(t, alice, "Cake")

	if len(r.Steps["main"]) != 2 || len(r.Ingredients) != 2 || r.Ingredients[0].Name != "flour" {
		t.Errorf("created recipe = %+v", r)
	}
}

func TestPostTextRecipeQueuesJob(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	drainRecipeQueue()

	rec := request(t, RecipesHandler, http.MethodPost, "/recipes", models.RawRecipe{Name: "Soup", Type: "text", Text: "Boil water."}, &alice)
	expectStatus(t, rec, http.StatusCreated)

	jobs := drainRecipeQueue()
	if len(jobs) != 1 {
		t.Fatalf("queued jobs = %d, want 1", len(jobs))
	}
	if job := jobs[0]; job.Name != "Soup" || job.Text != "Boil water." || job.Type != "text" || job.User_id != alice.ID {
		t.Errorf("queued job = %+v", job)
	}

	// the job is saved before it's queued, so it survives a restart while waiting in the queue
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipe_jobs WHERE id = $1 AND title = 'Soup' AND NOT parsed`, jobs[0].ID); n != 1 {
		t.Errorf("queued job %d has no unparsed recipe_jobs row", jobs[0].ID)
	}
}

func TestPostImageRecipeQueuesJob(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	drainRecipeQueue()

	rec := request(t, RecipesHandler, http.MethodPost, "/recipes", models.RawRecipe{Name: "Photo", Type: "image", Images: []string{"aW1n"}}, &alice)
	expectStatus(t, rec, http.StatusCreated)

	jobs := drainRecipeQueue()
	if len(jobs) != 1 || jobs[0].Type != "image" || len(jobs[0].Images) != 1 || jobs[0].User_id != alice.ID || jobs[0].ID == 0 {
		t.Fatalf("queued jobs = %+v", jobs)
	}
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipe_jobs WHERE id = $1 AND type = 'image'`, jobs[0].ID); n != 1 {
		t.Errorf("queued job %d has no recipe_jobs row", jobs[0].ID)
	}
}

func TestQueuedJobSurvivesRestart(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	drainRecipeQueue()

	rec := request(t, RecipesHandler, http.MethodPost, "/recipes", models.RawRecipe{Name: "Soup", Type: "text", Text: "Boil water."}, &alice)
	expectStatus(t, rec, http.StatusCreated)

	// simulate a restart: the in-memory queue is lost, then startup reloads unparsed jobs
	drainRecipeQueue()
	if err := lib.LoadUnparsedRecipeJobs(ctx); err != nil {
		t.Fatal(err)
	}

	jobs := drainRecipeQueue()
	if len(jobs) != 1 || jobs[0].Name != "Soup" {
		t.Errorf("jobs reloaded after restart = %+v, want the Soup job", jobs)
	}
}

func TestGetRecipes(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	bob := testutil.CreateUser(t, ctx, "bob")
	createManualRecipe(t, alice, "Alice's cake")
	createManualRecipe(t, bob, "Bob's bread")

	titles := func(rec *httptest.ResponseRecorder) []string {
		expectStatus(t, rec, http.StatusOK)
		var got []string
		for _, r := range decodeRecipes(t, rec) {
			got = append(got, r.Title)
		}
		return got
	}

	if got := titles(request(t, RecipesHandler, http.MethodGet, "/recipes", nil, &alice)); len(got) != 1 || got[0] != "Alice's cake" {
		t.Errorf("own recipes = %v", got)
	}

	// share link: anyone (even logged out) can view a user's recipes by their uuid
	if got := titles(request(t, RecipesHandler, http.MethodGet, "/recipes?recipes_of="+bob.UUID, nil, nil)); len(got) != 1 || got[0] != "Bob's bread" {
		t.Errorf("recipes_of bob = %v", got)
	}

	// unknown uuid falls back to your own recipes
	if got := titles(request(t, RecipesHandler, http.MethodGet, "/recipes?recipes_of=00000000-0000-0000-0000-000000000000", nil, &alice)); len(got) != 1 || got[0] != "Alice's cake" {
		t.Errorf("unknown recipes_of = %v", got)
	}
}

func TestGetRecipesEmptyIsArray(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	rec := request(t, RecipesHandler, http.MethodGet, "/recipes", nil, &alice)
	expectStatus(t, rec, http.StatusOK)
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("body = %s, want [] (the frontend reads .length)", body)
	}
}

func TestPatchRecipe(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := createManualRecipe(t, alice, "Cake")

	update := models.UpdateRecipeRequest{
		RecipeID:     r.ID,
		UpdatedSteps: []models.UpdatedStep{{StepName: "main", NewSteps: "Mix well.\nBake for 30 minutes."}},
		UpdatedIngredients: []models.UpdatedIngredient{
			{RecipeIngredientID: r.Ingredients[1].RecipeIngredientId, Name: "brown sugar", Amount: "3 tbsp"},
		},
	}
	expectStatus(t, request(t, RecipesHandler, http.MethodPatch, "/recipes", update, &alice), http.StatusOK)

	after := createdRecipe(t, alice, "Cake")
	if after.Steps["main"][1] != "Bake for 30 minutes." {
		t.Errorf("steps = %q", after.Steps)
	}
	if ing := after.Ingredients[1]; ing.Name != "brown sugar" || ing.Amount != "3 tbsp" {
		t.Errorf("ingredient = %+v", ing)
	}
}

func TestPatchRecipeAuth(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	mallory := testutil.CreateUser(t, ctx, "mallory")
	r := createManualRecipe(t, alice, "Cake")
	update := models.UpdateRecipeRequest{
		RecipeID:     r.ID,
		UpdatedSteps: []models.UpdatedStep{{StepName: "main", NewSteps: "Hacked."}},
	}

	expectStatus(t, request(t, RecipesHandler, http.MethodPatch, "/recipes", update, nil), http.StatusUnauthorized)
	expectStatus(t, request(t, RecipesHandler, http.MethodPatch, "/recipes", update, &mallory), http.StatusNotFound)
	expectStatus(t, request(t, RecipesHandler, http.MethodPatch, "/recipes", "{", &alice), http.StatusBadRequest)

	if after := createdRecipe(t, alice, "Cake"); after.Steps["main"][0] != "Mix." {
		t.Errorf("recipe was changed by an unauthorized request: %q", after.Steps)
	}
}

func TestGetRecipesIsMine(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	bob := testutil.CreateUser(t, ctx, "bob")
	testutil.Follow(t, ctx, alice, bob)
	createManualRecipe(t, alice, "Alice's cake")
	createManualRecipe(t, bob, "Bob's bread")

	isMine := func(rec *httptest.ResponseRecorder) map[string]bool {
		t.Helper()
		expectStatus(t, rec, http.StatusOK)
		got := map[string]bool{}
		for _, r := range decodeRecipes(t, rec) {
			got[r.Title] = r.IsMine
			if r.Tags == nil {
				t.Errorf("%s: tags is null, want []", r.Title)
			}
		}
		return got
	}

	want := map[string]bool{"Alice's cake": true, "Bob's bread": false}
	if got := isMine(request(t, RecipesHandler, http.MethodGet, "/recipes", nil, &alice)); !reflect.DeepEqual(got, want) {
		t.Errorf("alice's view = %v, want %v", got, want)
	}

	// viewing bob's share link: nothing is "mine", logged in or not
	if got := isMine(request(t, RecipesHandler, http.MethodGet, "/recipes?recipes_of="+bob.UUID, nil, &alice)); got["Bob's bread"] {
		t.Error("bob's recipe marked as alice's on his share link")
	}
	if got := isMine(request(t, RecipesHandler, http.MethodGet, "/recipes?recipes_of="+bob.UUID, nil, nil)); got["Bob's bread"] {
		t.Error("recipe marked as mine for a logged out visitor")
	}

	// the owner's database id isn't sent to the browser
	rec := request(t, RecipesHandler, http.MethodGet, "/recipes", nil, &alice)
	if strings.Contains(rec.Body.String(), "OwnerID") || strings.Contains(rec.Body.String(), "owner_id") {
		t.Error("owner id leaked in the response")
	}
}

func TestPatchRecipeTags(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := createManualRecipe(t, alice, "Cake")

	patch := func(tags []string) *httptest.ResponseRecorder {
		return request(t, RecipesHandler, http.MethodPatch, "/recipes",
			models.UpdateRecipeRequest{RecipeID: r.ID, Tags: &tags}, &alice)
	}

	expectStatus(t, patch([]string{"Baking", "dessert"}), http.StatusOK)
	if got := createdRecipe(t, alice, "Cake").Tags; !reflect.DeepEqual(got, []string{"baking", "dessert"}) {
		t.Errorf("tags = %q", got)
	}

	rec := patch([]string{strings.Repeat("x", 100)})
	expectStatus(t, rec, http.StatusBadRequest)
	if !strings.Contains(rec.Body.String(), "invalid tags") {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestPatchRecipeTitle(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := createManualRecipe(t, alice, "Cake")

	patch := func(title string) *httptest.ResponseRecorder {
		return request(t, RecipesHandler, http.MethodPatch, "/recipes",
			models.UpdateRecipeRequest{RecipeID: r.ID, Title: &title}, &alice)
	}

	expectStatus(t, patch("Lemon drizzle cake"), http.StatusOK)
	createdRecipe(t, alice, "Lemon drizzle cake")

	rec := patch("  ")
	expectStatus(t, rec, http.StatusBadRequest)
	if !strings.Contains(rec.Body.String(), "needs a title") {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestDeleteRecipe(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	mallory := testutil.CreateUser(t, ctx, "mallory")
	r := createManualRecipe(t, alice, "Cake")
	body := map[string]int{"recipe_id": r.ID}

	// someone else's delete is a no-op
	expectStatus(t, request(t, RecipesHandler, http.MethodDelete, "/recipes", body, &mallory), http.StatusOK)
	expectStatus(t, request(t, RecipesHandler, http.MethodDelete, "/recipes", body, nil), http.StatusOK)
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipes`); n != 1 {
		t.Fatal("recipe deleted by someone other than its owner")
	}

	expectStatus(t, request(t, RecipesHandler, http.MethodDelete, "/recipes", body, &alice), http.StatusOK)
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipes`); n != 0 {
		t.Error("owner's delete did not remove the recipe")
	}

	expectStatus(t, request(t, RecipesHandler, http.MethodDelete, "/recipes", "{", &alice), http.StatusBadRequest)
}

// createdRecipe fetches a recipe by title through GET /recipes.
func createdRecipe(t *testing.T, user testutil.User, title string) lib.RecipeResponse {
	t.Helper()

	for _, r := range decodeRecipes(t, request(t, RecipesHandler, http.MethodGet, "/recipes", nil, &user)) {
		if r.Title == title {
			return r
		}
	}
	t.Fatalf("recipe %q not found", title)
	return lib.RecipeResponse{}
}

// ---------------------------------------------------------------------------
// /ingredients
// ---------------------------------------------------------------------------

func TestIngredientsTagging(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := createManualRecipe(t, alice, "Cake")
	flourID := r.Ingredients[0].IngredientId

	tag := func(category, location, season string) {
		t.Helper()
		body := []IngredientUpdate{{ID: flourID, Category: category, Location: location, Season: season}}
		expectStatus(t, request(t, IngredientsHandler, http.MethodPost, "/ingredients", body, &alice), http.StatusOK)
	}
	flour := func() lib.IngredientWithTag {
		t.Helper()
		rec := request(t, IngredientsHandler, http.MethodGet, "/ingredients", nil, &alice)
		expectStatus(t, rec, http.StatusOK)
		var ingredients []lib.IngredientWithTag
		if err := json.Unmarshal(rec.Body.Bytes(), &ingredients); err != nil {
			t.Fatal(err)
		}
		for _, ing := range ingredients {
			if ing.ID == int64(flourID) {
				return ing
			}
		}
		t.Fatal("flour not returned")
		return lib.IngredientWithTag{}
	}

	tag("baking", "pantry", "all")
	if got := flour(); got.Category == nil || *got.Category != "baking" || *got.Location != "pantry" || *got.Season != "all" {
		t.Errorf("after tagging = %+v", got)
	}

	// tagging again updates the existing tag instead of failing on the unique constraint
	tag("baking", "aisle 4", "")
	if got := flour(); *got.Location != "aisle 4" || *got.Season != "all" {
		t.Errorf("after retagging = %+v, want location updated and season kept", got)
	}

	expectStatus(t, request(t, IngredientsHandler, http.MethodPost, "/ingredients", "{", &alice), http.StatusBadRequest)
	expectStatus(t, request(t, IngredientsHandler, http.MethodDelete, "/ingredients", nil, &alice), http.StatusMethodNotAllowed)
}

// ---------------------------------------------------------------------------
// users
// ---------------------------------------------------------------------------

func TestFollowNewUser(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	bob := testutil.CreateUser(t, ctx, "bob")
	createManualRecipe(t, bob, "Bob's bread")

	follow := func(code string, user *testutil.User) *httptest.ResponseRecorder {
		return request(t, FollowNewUser, http.MethodPost, "/users/follow-new-user", FollowUserRequest{FriendCode: code}, user)
	}

	expectStatus(t, follow(bob.UUID, nil), http.StatusUnauthorized)
	expectStatus(t, follow("not-a-uuid", &alice), http.StatusBadRequest)

	rec := follow("00000000-0000-0000-0000-000000000000", &alice)
	expectStatus(t, rec, http.StatusBadRequest)
	if !strings.Contains(rec.Body.String(), "user does not exist") {
		t.Errorf("body = %q", rec.Body.String())
	}

	rec = follow(alice.UUID, &alice)
	expectStatus(t, rec, http.StatusBadRequest)
	if !strings.Contains(rec.Body.String(), "cannot follow yourself") {
		t.Errorf("body = %q", rec.Body.String())
	}

	expectStatus(t, follow(bob.UUID, &alice), http.StatusOK)

	// alice now sees bob's recipes
	if got := decodeRecipes(t, request(t, RecipesHandler, http.MethodGet, "/recipes", nil, &alice)); len(got) != 1 || got[0].Title != "Bob's bread" {
		t.Errorf("alice's recipes after following bob = %+v", got)
	}
}

func TestMeHandler(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	expectStatus(t, request(t, MeHandler, http.MethodGet, "/auth/me", nil, nil), http.StatusUnauthorized)

	rec := request(t, MeHandler, http.MethodGet, "/auth/me", nil, &alice)
	expectStatus(t, rec, http.StatusOK)
	var me struct{ Uuid, DisplayName, Email string }
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.Uuid != alice.UUID || me.DisplayName != "alice" || me.Email != "alice@example.com" {
		t.Errorf("me = %+v", me)
	}
}

func TestLogoutHandler(t *testing.T) {
	alice := testutil.User{ID: 42}

	rec := request(t, LogoutHandler, http.MethodPost, "/auth/logout", nil, &alice)
	expectStatus(t, rec, http.StatusOK)

	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" && c.MaxAge < 0 {
			return
		}
	}
	t.Error("logout did not expire the session cookie")
}

// ---------------------------------------------------------------------------
// misc
// ---------------------------------------------------------------------------

func TestCookingHandler(t *testing.T) {
	rec := request(t, CookingHandler, http.MethodGet, "/cooking/Chicken%20Soup", nil, nil)
	expectStatus(t, rec, http.StatusOK)

	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["message"] != "Cooking Chicken Soup" {
		t.Errorf("message = %q", body["message"])
	}
}

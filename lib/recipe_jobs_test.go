package lib

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"testing"

	"go-guacamole/db"
	"go-guacamole/internal/testutil"
	"go-guacamole/models"
)

type jobRow struct {
	Parsed     bool
	FailCount  int
	ParsedJSON []byte
}

func loadJob(t *testing.T, ctx context.Context, jobID int) jobRow {
	t.Helper()

	var row jobRow
	err := db.Pool.QueryRow(ctx,
		`SELECT parsed, fail_count, parsed_json FROM recipe_jobs WHERE id = $1`, jobID,
	).Scan(&row.Parsed, &row.FailCount, &row.ParsedJSON)
	if err != nil {
		t.Fatalf("loading job %d: %v", jobID, err)
	}
	return row
}

// onlyJobID returns the id of the single recipe_jobs row.
func onlyJobID(t *testing.T, ctx context.Context) int {
	t.Helper()

	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipe_jobs`); n != 1 {
		t.Fatalf("recipe_jobs rows = %d, want 1", n)
	}
	var id int
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM recipe_jobs`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCreateRecipeJob(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	jobID, err := CreateRecipeJob(ctx, models.RecipeJob{
		Name: "Photo recipe", Images: []string{"aW1n"}, Type: "image", User_id: alice.ID,
	})
	if err != nil {
		t.Fatalf("CreateRecipeJob: %v", err)
	}

	var title, jobType string
	var images []string
	var userID int
	err = db.Pool.QueryRow(ctx,
		`SELECT title, type, images, user_id FROM recipe_jobs WHERE id = $1`, jobID,
	).Scan(&title, &jobType, &images, &userID)
	if err != nil {
		t.Fatal(err)
	}
	if title != "Photo recipe" || jobType != "image" || !reflect.DeepEqual(images, []string{"aW1n"}) || userID != alice.ID {
		t.Errorf("job = %q %q %v %d", title, jobType, images, userID)
	}

	if row := loadJob(t, ctx, jobID); row.Parsed || row.FailCount != 0 || row.ParsedJSON != nil {
		t.Errorf("new job = %+v, want unparsed with no failures or parsed_json", row)
	}
}

func TestRecipeJobParsedJSONAndFailCount(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	jobID, err := CreateRecipeJob(ctx, models.RecipeJob{Name: "Chicken", Text: "...", Type: "text", User_id: alice.ID})
	if err != nil {
		t.Fatal(err)
	}

	stored, _ := json.Marshal(sampleParsed())
	if err := SaveRecipeJobParsedJSON(ctx, jobID, stored); err != nil {
		t.Fatalf("SaveRecipeJobParsedJSON: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := IncrementRecipeJobFailCount(ctx, jobID); err != nil {
			t.Fatalf("IncrementRecipeJobFailCount: %v", err)
		}
	}

	row := loadJob(t, ctx, jobID)
	if row.FailCount != 2 {
		t.Errorf("fail_count = %d, want 2", row.FailCount)
	}
	// jsonb doesn't keep key order, so compare decoded values
	var roundTripped RecipeParsed
	if err := json.Unmarshal(row.ParsedJSON, &roundTripped); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&roundTripped, sampleParsed()) {
		t.Errorf("parsed_json round trip = %+v", roundTripped)
	}
}

func TestLoadUnparsedRecipeJobs(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	drainRecipeQueue()
	t.Cleanup(func() { drainRecipeQueue() })

	newJob := func(name string) int {
		id, err := CreateRecipeJob(ctx, models.RecipeJob{Name: name, Text: "...", Type: "text", User_id: alice.ID})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	withJSON := newJob("has parsed json")
	stored, _ := json.Marshal(sampleParsed())
	if err := SaveRecipeJobParsedJSON(ctx, withJSON, stored); err != nil {
		t.Fatal(err)
	}

	failedTwice := newJob("failed twice")
	parsed := newJob("already parsed")
	failedOut := newJob("failed too often")

	_, err := db.Pool.Exec(ctx, `UPDATE recipe_jobs SET fail_count = 2 WHERE id = $1`, failedTwice)
	if err == nil {
		_, err = db.Pool.Exec(ctx, `UPDATE recipe_jobs SET parsed = TRUE WHERE id = $1`, parsed)
	}
	if err == nil {
		_, err = db.Pool.Exec(ctx, `UPDATE recipe_jobs SET fail_count = $1 WHERE id = $2`, MaxRecipeJobFailures, failedOut)
	}
	if err != nil {
		t.Fatal(err)
	}

	if err := LoadUnparsedRecipeJobs(ctx); err != nil {
		t.Fatalf("LoadUnparsedRecipeJobs: %v", err)
	}

	queued := drainRecipeQueue()
	var names []string
	for _, job := range queued {
		names = append(names, job.Name)
		if job.ID == withJSON && len(job.ParsedJSON) == 0 {
			t.Error("stored parsed_json was not loaded onto the job")
		}
		if job.User_id != alice.ID || job.Type != "text" {
			t.Errorf("queued job = %+v", job)
		}
	}
	sort.Strings(names)
	if want := []string{"failed twice", "has parsed json"}; !reflect.DeepEqual(names, want) {
		t.Errorf("queued = %v, want %v", names, want)
	}
}

// ---------------------------------------------------------------------------
// ProcessRecipeJob: the whole AI flow with a fake DeepInfra
// ---------------------------------------------------------------------------

func TestProcessRecipeJobTextSuccess(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	calls := fakeDeepInfra(t, modelReplies(sampleModelJSON))

	job := models.RecipeJob{Name: "Honey soy chicken", Text: "raw recipe text", Type: "text", User_id: alice.ID}
	if err := ProcessRecipeJob(ctx, job); err != nil {
		t.Fatalf("ProcessRecipeJob: %v", err)
	}

	if calls.Load() != 1 {
		t.Errorf("DeepInfra calls = %d, want 1", calls.Load())
	}

	r := recipeByTitle(t, ctx, alice.ID, "Honey soy chicken")
	if len(r.Steps["main"]) != 3 || len(r.Steps["sauce"]) != 1 {
		t.Errorf("steps = %q", r.Steps)
	}
	gotComponents := map[string]string{}
	for _, ing := range r.Ingredients {
		gotComponents[ing.Name] = ing.Component
	}
	wantComponents := map[string]string{"chicken thighs": "main", "salt": "main", "soy sauce": "sauce", "honey": "sauce"}
	if !reflect.DeepEqual(gotComponents, wantComponents) {
		t.Errorf("components = %v, want %v", gotComponents, wantComponents)
	}

	row := loadJob(t, ctx, onlyJobID(t, ctx))
	if !row.Parsed || row.FailCount != 0 || row.ParsedJSON == nil {
		t.Errorf("job after success = parsed:%v fail_count:%d parsed_json set:%v", row.Parsed, row.FailCount, row.ParsedJSON != nil)
	}

	// a restart must not queue the finished job again
	drainRecipeQueue()
	if err := LoadUnparsedRecipeJobs(ctx); err != nil {
		t.Fatal(err)
	}
	if jobs := drainRecipeQueue(); len(jobs) != 0 {
		t.Errorf("finished job was queued again after restart: %+v", jobs)
	}
}

func TestProcessRecipeJobReusesStoredParse(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	calls := fakeDeepInfra(t, modelMustNotBeCalled(t))

	job := models.RecipeJob{Name: "Chicken", Text: "...", Type: "text", User_id: alice.ID}
	jobID, err := CreateRecipeJob(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	job.ID = jobID
	job.ParsedJSON, _ = json.Marshal(sampleParsed())

	if err := ProcessRecipeJob(ctx, job); err != nil {
		t.Fatalf("ProcessRecipeJob: %v", err)
	}

	if calls.Load() != 0 {
		t.Errorf("DeepInfra calls = %d, want 0 (stored parse should be reused)", calls.Load())
	}
	if r := recipeByTitle(t, ctx, alice.ID, "Chicken"); len(r.Ingredients) != 4 {
		t.Errorf("ingredients = %d, want 4", len(r.Ingredients))
	}
	if !loadJob(t, ctx, jobID).Parsed {
		t.Error("job not marked parsed")
	}
}

func TestProcessRecipeJobModelFailures(t *testing.T) {
	tests := []struct {
		name    string
		respond http.HandlerFunc
	}{
		{"server error", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}},
		{"invalid json", modelReplies("not json at all")},
		{"empty recipe", modelReplies(`{"steps": {}, "ingredients": []}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testutil.SetupDB(t)
			alice := testutil.CreateUser(t, ctx, "alice")
			fakeDeepInfra(t, tt.respond)

			job := models.RecipeJob{Name: "Chicken", Text: "...", Type: "text", User_id: alice.ID}
			if err := ProcessRecipeJob(ctx, job); err == nil {
				t.Fatal("expected ProcessRecipeJob to fail")
			}

			row := loadJob(t, ctx, onlyJobID(t, ctx))
			if row.Parsed || row.FailCount != 1 || row.ParsedJSON != nil {
				t.Errorf("job = parsed:%v fail_count:%d parsed_json:%s, want unparsed, 1 failure, nothing stored",
					row.Parsed, row.FailCount, row.ParsedJSON)
			}
			if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipes`); n != 0 {
				t.Errorf("recipes = %d, want 0", n)
			}
		})
	}
}

// A save failure after a good model response keeps the parse result, and the retry reuses it
// without calling the model again.
func TestProcessRecipeJobRetryAfterSaveFailure(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	drainRecipeQueue()
	t.Cleanup(func() { drainRecipeQueue() })

	// make saving this recipe fail until the constraint is dropped
	dropConstraint := func() {
		db.Pool.Exec(context.Background(), `ALTER TABLE recipes DROP CONSTRAINT IF EXISTS test_block_save`)
	}
	t.Cleanup(dropConstraint)
	if _, err := db.Pool.Exec(ctx, `ALTER TABLE recipes ADD CONSTRAINT test_block_save CHECK (title <> 'Blocked')`); err != nil {
		t.Fatal(err)
	}

	calls := fakeDeepInfra(t, modelReplies(sampleModelJSON))
	job := models.RecipeJob{Name: "Blocked", Text: "...", Type: "text", User_id: alice.ID}
	if err := ProcessRecipeJob(ctx, job); err == nil {
		t.Fatal("expected the save to fail")
	}

	jobID := onlyJobID(t, ctx)
	row := loadJob(t, ctx, jobID)
	if row.Parsed || row.FailCount != 1 || row.ParsedJSON == nil {
		t.Fatalf("after save failure = parsed:%v fail_count:%d parsed_json set:%v, want parse result kept",
			row.Parsed, row.FailCount, row.ParsedJSON != nil)
	}

	// retry the way a restart does: reload unparsed jobs from the database
	dropConstraint()
	if err := LoadUnparsedRecipeJobs(ctx); err != nil {
		t.Fatal(err)
	}
	queued := drainRecipeQueue()
	if len(queued) != 1 {
		t.Fatalf("queued jobs = %d, want 1", len(queued))
	}
	if err := ProcessRecipeJob(ctx, queued[0]); err != nil {
		t.Fatalf("retry: %v", err)
	}

	if calls.Load() != 1 {
		t.Errorf("DeepInfra calls = %d, want 1 (retry should reuse the stored parse)", calls.Load())
	}
	if !loadJob(t, ctx, jobID).Parsed {
		t.Error("job not marked parsed after retry")
	}
	if r := recipeByTitle(t, ctx, alice.ID, "Blocked"); len(r.Ingredients) != 4 {
		t.Errorf("ingredients = %d, want 4", len(r.Ingredients))
	}
}

func TestProcessRecipeJobImage(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	var models_ []string
	calls := fakeDeepInfra(t, func(w http.ResponseWriter, r *http.Request) {
		var req diChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		models_ = append(models_, req.Model)
		if req.Model == deepInfraVisionModel {
			writeChatCompletion(w, "transcribed recipe text")
			return
		}
		writeChatCompletion(w, sampleModelJSON)
	})

	job := models.RecipeJob{Name: "From a photo", Images: []string{"aW1n"}, Type: "image", User_id: alice.ID}
	if err := ProcessRecipeJob(ctx, job); err != nil {
		t.Fatalf("ProcessRecipeJob: %v", err)
	}

	if calls.Load() != 2 || models_[0] != deepInfraVisionModel || models_[1] != deepInfraTextModel {
		t.Errorf("DeepInfra calls = %v, want vision then text model", models_)
	}
	if r := recipeByTitle(t, ctx, alice.ID, "From a photo"); len(r.Ingredients) != 4 {
		t.Errorf("ingredients = %d, want 4", len(r.Ingredients))
	}
	if got := jobTranscript(t, ctx, onlyJobID(t, ctx)); got != "transcribed recipe text" {
		t.Errorf("stored transcript = %q", got)
	}
}

func jobTranscript(t *testing.T, ctx context.Context, jobID int) string {
	t.Helper()

	var transcript *string
	if err := db.Pool.QueryRow(ctx, `SELECT transcript FROM recipe_jobs WHERE id = $1`, jobID).Scan(&transcript); err != nil {
		t.Fatal(err)
	}
	if transcript == nil {
		return ""
	}
	return *transcript
}

// fakeImageModels answers vision requests with transcript and text requests with textReply(), and
// counts vision calls.
func fakeImageModels(t *testing.T, transcript string, textReply func() string) *int {
	visionCalls := new(int)
	fakeDeepInfra(t, func(w http.ResponseWriter, r *http.Request) {
		var req diChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model == deepInfraVisionModel {
			*visionCalls++
			writeChatCompletion(w, transcript)
			return
		}
		writeChatCompletion(w, textReply())
	})
	return visionCalls
}

// A failure in the text step keeps the transcript, and the retry skips the vision model.
func TestProcessRecipeJobImageRetryReusesTranscript(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	drainRecipeQueue()
	t.Cleanup(func() { drainRecipeQueue() })

	textReply := "not json" // first text call fails
	visionCalls := fakeImageModels(t, "transcribed recipe text", func() string { return textReply })

	job := models.RecipeJob{Name: "From a photo", Images: []string{"aW1n"}, Type: "image", User_id: alice.ID}
	if err := ProcessRecipeJob(ctx, job); err == nil {
		t.Fatal("expected the text step to fail")
	}

	jobID := onlyJobID(t, ctx)
	if got := jobTranscript(t, ctx, jobID); got != "transcribed recipe text" {
		t.Errorf("transcript after text failure = %q, want it kept", got)
	}
	if row := loadJob(t, ctx, jobID); row.FailCount != 1 || row.ParsedJSON != nil {
		t.Errorf("job = fail_count:%d parsed_json:%s", row.FailCount, row.ParsedJSON)
	}

	// restart: the reloaded job carries the transcript, so only the text model runs
	textReply = sampleModelJSON
	if err := LoadUnparsedRecipeJobs(ctx); err != nil {
		t.Fatal(err)
	}
	queued := drainRecipeQueue()
	if len(queued) != 1 || queued[0].Transcript != "transcribed recipe text" {
		t.Fatalf("reloaded jobs = %+v, want the job with its transcript", queued)
	}
	if err := ProcessRecipeJob(ctx, queued[0]); err != nil {
		t.Fatalf("retry: %v", err)
	}

	if *visionCalls != 1 {
		t.Errorf("vision model calls = %d, want 1 (retry should reuse the transcript)", *visionCalls)
	}
	if !loadJob(t, ctx, jobID).Parsed {
		t.Error("job not marked parsed after retry")
	}
}

func TestProcessRecipeJobImageEmptyTranscript(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	fakeImageModels(t, "   ", func() string {
		t.Error("text model should not run on an empty transcript")
		return sampleModelJSON
	})

	job := models.RecipeJob{Name: "Blurry photo", Images: []string{"aW1n"}, Type: "image", User_id: alice.ID}
	if err := ProcessRecipeJob(ctx, job); err == nil {
		t.Fatal("expected an error for an empty transcript")
	}

	jobID := onlyJobID(t, ctx)
	if got := jobTranscript(t, ctx, jobID); got != "" {
		t.Errorf("stored transcript = %q, want nothing saved", got)
	}
	if loadJob(t, ctx, jobID).FailCount != 1 {
		t.Error("fail_count not incremented")
	}
}

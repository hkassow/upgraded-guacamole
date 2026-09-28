package lib

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"go-guacamole/models"
)

func TestMain(m *testing.M) {
	// Tests must never reach the real DeepInfra API. Anything that doesn't install a fake with
	// fakeDeepInfra fails fast against this unreachable address instead.
	deepInfraChatURL = "http://127.0.0.1:0/deepinfra-disabled-in-tests"
	os.Exit(m.Run())
}

// fakeDeepInfra points callDeepInfra at a local server that answers with respond, and returns a
// counter of how many requests it received.
func fakeDeepInfra(t *testing.T, respond http.HandlerFunc) *atomic.Int32 {
	t.Helper()

	calls := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		respond(w, r)
	}))
	t.Cleanup(srv.Close)

	previous := deepInfraChatURL
	deepInfraChatURL = srv.URL
	t.Cleanup(func() { deepInfraChatURL = previous })

	t.Setenv("DEEPINFRA_API_KEY_FILE", "")
	t.Setenv("DEEPINFRA_API_KEY", "test-key")
	return calls
}

// modelReplies answers every request with a chat completion whose content is content.
func modelReplies(content string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeChatCompletion(w, content)
	}
}

// writeChatCompletion streams content back the way DeepInfra does: server-sent events with the
// text split across several chunks, then a finish chunk, a usage chunk and [DONE].
func writeChatCompletion(w http.ResponseWriter, content string) {
	writeStreamChunks(w, content, "stop")
}

func writeStreamChunks(w http.ResponseWriter, content, finishReason string) {
	w.Header().Set("Content-Type", "text/event-stream")

	runes := []rune(content) // split on characters so "½" etc. aren't cut in half
	third := len(runes) / 3
	parts := []string{string(runes[:third]), string(runes[third : 2*third]), string(runes[2*third:])}
	for _, part := range parts {
		writeEvent(w, map[string]any{"choices": []map[string]any{{"delta": map[string]any{"content": part}}}})
	}
	writeEvent(w, map[string]any{"choices": []map[string]any{{"delta": map[string]any{}, "finish_reason": finishReason}}})
	writeEvent(w, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 50}})
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func writeEvent(w http.ResponseWriter, event any) {
	b, _ := json.Marshal(event)
	fmt.Fprintf(w, "data: %s\n\n", b)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// shortDeepInfraTimeouts shrinks the stream timeouts so stall tests run quickly.
func shortDeepInfraTimeouts(t *testing.T) {
	firstData, idle, maxDuration := deepInfraFirstDataTimeout, deepInfraIdleTimeout, deepInfraMaxDuration
	deepInfraFirstDataTimeout, deepInfraIdleTimeout, deepInfraMaxDuration = 100*time.Millisecond, 100*time.Millisecond, 5*time.Second
	t.Cleanup(func() {
		deepInfraFirstDataTimeout, deepInfraIdleTimeout, deepInfraMaxDuration = firstData, idle, maxDuration
	})
}

// modelMustNotBeCalled fails the test if DeepInfra is called.
func modelMustNotBeCalled(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("DeepInfra was called but should not have been")
		http.Error(w, "unexpected call", http.StatusInternalServerError)
	}
}

// sampleModelJSON is a realistic model response: two sections, a mis-cased component ("Sauce"),
// a missing component that has to be inferred from the step ingredients (honey), and a code fence.
const sampleModelJSON = "```json\n" + `{
  "steps": {
    "main": ["Season the chicken.", "Roast for 30 minutes.", "Pour the sauce over the chicken."],
    "sauce": ["Whisk the soy sauce and honey together."]
  },
  "ingredients": [
    {"name": "chicken thighs", "amount": "500g", "alt_amount": "1 lb", "preparation_notes": "boneless", "component": "main"},
    {"name": "salt", "amount": "1 tsp", "alt_amount": "", "preparation_notes": "", "component": "main"},
    {"name": "soy sauce", "amount": "2 tbsp", "alt_amount": "", "preparation_notes": "", "component": "Sauce"},
    {"name": "honey", "amount": "1 tbsp", "alt_amount": "", "preparation_notes": "", "component": ""}
  ],
  "ingredients_used_for_step": {
    "main": {"1": [{"name": "chicken thighs", "amount": "500g"}, {"name": "salt", "amount": "1 tsp"}]},
    "sauce": {"1": [{"name": "soy sauce", "amount": "2 tbsp"}, {"name": "honey", "amount": "1 tbsp"}]}
  }
}` + "\n```"

// sampleParsed is a parsed recipe with two sections, used to seed the database.
func sampleParsed() *RecipeParsed {
	return &RecipeParsed{
		Steps: map[string][]string{
			"main":  {"Season the chicken.", "Roast for 30 minutes."},
			"sauce": {"Whisk the soy sauce and honey together."},
		},
		Ingredients: []Ingredient{
			{Name: "chicken thighs", Amount: "500g", AltAmount: "1 lb", PreparationNotes: "boneless", Component: "main"},
			{Name: "salt", Amount: "1 tsp", Component: "main"},
			{Name: "soy sauce", Amount: "2 tbsp", Component: "sauce"},
			{Name: "honey", Amount: "1 tbsp", Component: "sauce"},
		},
		IngredientsUsedForStep: map[string]map[string][]StepIngredient{
			"main":  {"1": {{Name: "chicken thighs", Amount: "500g"}, {Name: "salt", Amount: "1 tsp"}}},
			"sauce": {"1": {{Name: "soy sauce", Amount: "2 tbsp"}, {Name: "honey", Amount: "1 tbsp"}}},
		},
	}
}

// drainRecipeQueue empties RecipeQueue and returns whatever was in it.
func drainRecipeQueue() []models.RecipeJob {
	var jobs []models.RecipeJob
	for {
		select {
		case job := <-RecipeQueue:
			jobs = append(jobs, job)
		default:
			return jobs
		}
	}
}

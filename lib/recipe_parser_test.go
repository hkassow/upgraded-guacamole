package lib

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCleanStepLines(t *testing.T) {
	got := cleanStepLines("  Preheat the oven.  \n\n\r\n   \nMix everything.\n\tBake.\t\n")
	want := []string{"Preheat the oven.", "Mix everything.", "Bake."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cleanStepLines = %q, want %q", got, want)
	}

	if got := cleanStepLines("   \n\n"); len(got) != 0 {
		t.Errorf("cleanStepLines of blank text = %q, want empty", got)
	}
}

func TestCleanupJSON(t *testing.T) {
	tests := map[string]string{
		"```json\n{\"a\":1}\n```": `{"a":1}`,
		"```\n{\"a\":1}\n```":     `{"a":1}`,
		"  {\"a\":1}  ":           `{"a":1}`,
	}
	for in, want := range tests {
		if got := cleanupJSON(in); got != want {
			t.Errorf("cleanupJSON(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeIngredientComponents(t *testing.T) {
	parsed := &RecipeParsed{
		Steps: map[string][]string{"main": {"a"}, "sauce": {"b"}},
		Ingredients: []Ingredient{
			{Name: "chicken", Component: "main"},
			{Name: "soy sauce", Component: " Sauce "}, // case/space differences are fixed
			{Name: "honey", Component: "glaze"},       // unknown, only used in sauce steps -> sauce
			{Name: "Salt"},                            // used in both sections -> main
			{Name: "pepper"},                          // no information -> main
		},
		IngredientsUsedForStep: map[string]map[string][]StepIngredient{
			"main":  {"1": {{Name: "salt"}}},
			"sauce": {"1": {{Name: "Honey"}, {Name: "salt"}}},
			"glaze": {"1": {{Name: "pepper"}}}, // section not in steps is ignored
		},
	}

	normalizeIngredientComponents(parsed)

	want := []string{"main", "sauce", "sauce", "main", "main"}
	for i, ing := range parsed.Ingredients {
		if ing.Component != want[i] {
			t.Errorf("%s: component = %q, want %q", ing.Name, ing.Component, want[i])
		}
	}
}

func TestNormalizeIngredientComponentsManualRecipe(t *testing.T) {
	// manual recipes have no components or step ingredients at all
	parsed := &RecipeParsed{
		Steps:       map[string][]string{"main": {"a"}},
		Ingredients: []Ingredient{{Name: "flour"}, {Name: "eggs"}},
	}

	normalizeIngredientComponents(parsed)

	for _, ing := range parsed.Ingredients {
		if ing.Component != "main" {
			t.Errorf("%s: component = %q, want main", ing.Name, ing.Component)
		}
	}
}

func TestRenameStepIngredient(t *testing.T) {
	stepIngredients := map[string]map[string][]StepIngredient{
		"main":  {"1": {{Name: "Butter", Amount: "1 tbsp"}, {Name: "flour", Amount: "1 cup"}}},
		"sauce": {"2": {{Name: " butter ", Amount: "2 tbsp"}}},
	}

	if !renameStepIngredient(stepIngredients, "butter", "ghee") {
		t.Fatal("renameStepIngredient returned false, want true")
	}

	want := map[string]map[string][]StepIngredient{
		"main":  {"1": {{Name: "ghee", Amount: "1 tbsp"}, {Name: "flour", Amount: "1 cup"}}},
		"sauce": {"2": {{Name: "ghee", Amount: "2 tbsp"}}},
	}
	if !reflect.DeepEqual(stepIngredients, want) {
		t.Errorf("after rename = %+v, want %+v", stepIngredients, want)
	}

	if renameStepIngredient(stepIngredients, "sugar", "honey") {
		t.Error("renaming an ingredient that isn't used returned true")
	}
	if renameStepIngredient(nil, "sugar", "honey") {
		t.Error("renaming in a nil map returned true")
	}
}

func TestCachedRecipeParse(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		usable bool
	}{
		{"nothing stored", "", false},
		{"invalid json", "{not json", false},
		{"json null", "null", false},
		{"no steps or ingredients", `{"steps":{},"ingredients":[]}`, false},
		{"usable", `{"steps":{"main":["Bake."]},"ingredients":[{"name":"flour"}]}`, true},
		{"ingredients only", `{"ingredients":[{"name":"flour"}]}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cachedRecipeParse(1, []byte(tt.raw))
			if (got != nil) != tt.usable {
				t.Errorf("cachedRecipeParse(%q) = %+v, want usable=%v", tt.raw, got, tt.usable)
			}
		})
	}
}

func TestExtractRecipeJSON(t *testing.T) {
	var request diChatRequest
	var authHeader string
	fakeDeepInfra(t, func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		writeChatCompletion(w, sampleModelJSON)
	})

	parsed, err := extractRecipeJSON("Chicken with honey soy sauce ...")
	if err != nil {
		t.Fatalf("extractRecipeJSON: %v", err)
	}

	if authHeader != "Bearer test-key" {
		t.Errorf("Authorization header = %q", authHeader)
	}
	if request.Model != deepInfraTextModel {
		t.Errorf("model = %q, want %q", request.Model, deepInfraTextModel)
	}
	if len(request.Messages) != 2 || request.Messages[0].Content != recipeTextSystemPrompt ||
		request.Messages[1].Content != "Chicken with honey soy sauce ..." {
		t.Errorf("unexpected messages: %+v", request.Messages)
	}

	if got := len(parsed.Steps["main"]); got != 3 {
		t.Errorf("main steps = %d, want 3", got)
	}
	if got := len(parsed.Ingredients); got != 4 {
		t.Errorf("ingredients = %d, want 4", got)
	}
	if got := parsed.Ingredients[0].AltAmount; got != "1 lb" {
		t.Errorf("alt_amount = %q, want 1 lb", got)
	}
	if got := len(parsed.IngredientsUsedForStep["sauce"]["1"]); got != 2 {
		t.Errorf("sauce step 1 ingredients = %d, want 2", got)
	}
}

func TestExtractRecipeJSONIgnoresMalformedStepIngredients(t *testing.T) {
	fakeDeepInfra(t, modelReplies(`{
		"steps": {"main": ["Bake."]},
		"ingredients": [{"name": "flour", "amount": "1 cup"}],
		"ingredients_used_for_step": ["not", "a", "map"]
	}`))

	parsed, err := extractRecipeJSON("recipe")
	if err != nil {
		t.Fatalf("extractRecipeJSON: %v", err)
	}
	if parsed.IngredientsUsedForStep != nil {
		t.Errorf("IngredientsUsedForStep = %+v, want nil", parsed.IngredientsUsedForStep)
	}
	if len(parsed.Ingredients) != 1 || len(parsed.Steps["main"]) != 1 {
		t.Errorf("rest of the recipe was not parsed: %+v", parsed)
	}
}

func TestExtractRecipeJSONErrors(t *testing.T) {
	tests := []struct {
		name      string
		respond   http.HandlerFunc
		wantInErr string
	}{
		{
			name:      "invalid recipe json",
			respond:   modelReplies("Sorry, I can't help with that."),
			wantInErr: "failed to parse recipe json",
		},
		{
			name: "non-200 status",
			respond: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "rate limited", http.StatusTooManyRequests)
			},
			wantInErr: "status 429",
		},
		{
			name: "error in body",
			respond: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"error": {"message": "model overloaded"}}`))
			},
			wantInErr: "model overloaded",
		},
		{
			name: "no choices",
			respond: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"choices": []}`))
			},
			wantInErr: "no choices",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeDeepInfra(t, tt.respond)

			_, err := extractRecipeJSON("recipe")
			if err == nil || !strings.Contains(err.Error(), tt.wantInErr) {
				t.Errorf("error = %v, want it to contain %q", err, tt.wantInErr)
			}
		})
	}
}

func TestParseRecipeImageCallDeepInfra(t *testing.T) {
	var requests []diChatRequest
	fakeDeepInfra(t, func(w http.ResponseWriter, r *http.Request) {
		var req diChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		requests = append(requests, req)

		if len(requests) == 1 {
			writeChatCompletion(w, "Honey soy chicken\n500g chicken thighs\n...")
			return
		}
		writeChatCompletion(w, sampleModelJSON)
	})

	parsed, err := ParseRecipeImageCallDeepInfra([]string{"aW1hZ2Ux", "aW1hZ2Uy"})
	if err != nil {
		t.Fatalf("ParseRecipeImageCallDeepInfra: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("DeepInfra calls = %d, want 2 (transcribe, then extract)", len(requests))
	}
	if requests[0].Model != deepInfraVisionModel {
		t.Errorf("first call model = %q, want vision model", requests[0].Model)
	}
	// user message content is [image, image, text] for two images
	if parts, ok := requests[0].Messages[1].Content.([]any); !ok || len(parts) != 3 {
		t.Errorf("vision request content = %#v, want 3 parts", requests[0].Messages[1].Content)
	}
	if requests[1].Model != deepInfraTextModel || requests[1].Messages[1].Content != "Honey soy chicken\n500g chicken thighs\n..." {
		t.Errorf("second call should send the transcript to the text model, got %+v", requests[1])
	}
	if len(parsed.Ingredients) != 4 {
		t.Errorf("ingredients = %d, want 4", len(parsed.Ingredients))
	}
}

func TestParseRecipeImageCallDeepInfraNoImages(t *testing.T) {
	calls := fakeDeepInfra(t, modelMustNotBeCalled(t))

	if _, err := ParseRecipeImageCallDeepInfra(nil); err == nil {
		t.Error("expected an error for no images")
	}
	if calls.Load() != 0 {
		t.Errorf("DeepInfra calls = %d, want 0", calls.Load())
	}
}

func TestLoadSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("  from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TEST_SECRET_FILE", path)
	t.Setenv("TEST_SECRET", "from-env")
	if got, err := LoadSecret("TEST_SECRET"); err != nil || got != "from-file" {
		t.Errorf("with file set: got %q, %v; want from-file", got, err)
	}

	t.Setenv("TEST_SECRET_FILE", "")
	if got, err := LoadSecret("TEST_SECRET"); err != nil || got != "from-env" {
		t.Errorf("env only: got %q, %v; want from-env", got, err)
	}

	t.Setenv("TEST_SECRET", "")
	if _, err := LoadSecret("TEST_SECRET"); err == nil {
		t.Error("missing secret: expected an error")
	}

	t.Setenv("TEST_SECRET_FILE", filepath.Join(t.TempDir(), "missing"))
	if _, err := LoadSecret("TEST_SECRET"); err == nil {
		t.Error("missing secret file: expected an error")
	}
}

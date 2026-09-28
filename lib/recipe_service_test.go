package lib

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"go-guacamole/db"
	"go-guacamole/internal/testutil"
	"go-guacamole/models"
)

// recipeByTitle loads the recipes visible to userID and returns the one with the given title.
func recipeByTitle(t *testing.T, ctx context.Context, userID int, title string) RecipeResponse {
	t.Helper()

	recipes, err := GetAllRecipes(ctx, userID)
	if err != nil {
		t.Fatalf("GetAllRecipes: %v", err)
	}
	for _, r := range recipes {
		if r.Title == title {
			return r
		}
	}
	t.Fatalf("recipe %q not visible to user %d", title, userID)
	return RecipeResponse{}
}

func ingredientNames(ings []ParsedIngredient) []string {
	names := make([]string, len(ings))
	for i, ing := range ings {
		names[i] = ing.Name
	}
	return names
}

// saveSample saves sampleParsed() for user and returns the saved recipe.
func saveSample(t *testing.T, ctx context.Context, user testutil.User, title string) RecipeResponse {
	t.Helper()

	if err := SaveParsedRecipe(ctx, title, user.ID, sampleParsed(), 0); err != nil {
		t.Fatalf("SaveParsedRecipe: %v", err)
	}
	return recipeByTitle(t, ctx, user.ID, title)
}

// ---------------------------------------------------------------------------
// creating recipes
// ---------------------------------------------------------------------------

func TestHandleManualRecipePost(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	err := HandleManualRecipePost(ctx, alice.ID, models.RawRecipe{
		Name: "Pancakes",
		Text: "  Mix the flour and eggs.  \n\n\n  Fry in butter.\n",
		Ingredients: []models.RawIngredient{
			{Name: "flour", Amount: "1 cup", PreparationNotes: "sifted"},
			{Name: "eggs", Amount: "2"},
			{Name: "butter", Amount: "1 tbsp"},
		},
	})
	if err != nil {
		t.Fatalf("HandleManualRecipePost: %v", err)
	}

	r := recipeByTitle(t, ctx, alice.ID, "Pancakes")

	wantSteps := map[string][]string{"main": {"Mix the flour and eggs.", "Fry in butter."}}
	if !reflect.DeepEqual(r.Steps, wantSteps) {
		t.Errorf("steps = %q, want %q", r.Steps, wantSteps)
	}
	if got := ingredientNames(r.Ingredients); !reflect.DeepEqual(got, []string{"flour", "eggs", "butter"}) {
		t.Errorf("ingredients = %v, want entry order kept", got)
	}
	flour := r.Ingredients[0]
	if flour.Amount != "1 cup" || flour.PreparationNotes != "sifted" || flour.Component != "main" {
		t.Errorf("flour = %+v", flour)
	}
	if r.StepIngredients != nil {
		t.Errorf("manual recipe step ingredients = %+v, want none", r.StepIngredients)
	}
}

func TestSaveParsedRecipe(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	r := saveSample(t, ctx, alice, "Honey soy chicken")

	if !reflect.DeepEqual(r.Steps, sampleParsed().Steps) {
		t.Errorf("steps = %q", r.Steps)
	}
	if !reflect.DeepEqual(r.StepIngredients, sampleParsed().IngredientsUsedForStep) {
		t.Errorf("step ingredients = %+v", r.StepIngredients)
	}

	wantComponents := map[string]string{"chicken thighs": "main", "salt": "main", "soy sauce": "sauce", "honey": "sauce"}
	for _, ing := range r.Ingredients {
		if ing.Component != wantComponents[ing.Name] {
			t.Errorf("%s component = %q, want %q", ing.Name, ing.Component, wantComponents[ing.Name])
		}
	}
	if chicken := r.Ingredients[0]; chicken.AltAmount != "1 lb" || chicken.PreparationNotes != "boneless" {
		t.Errorf("chicken = %+v", chicken)
	}
}

func TestSaveParsedRecipeNormalizesComponents(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	parsed := sampleParsed()
	parsed.Ingredients[2].Component = "SAUCE" // wrong case
	parsed.Ingredients[3].Component = ""      // missing, inferred from step ingredients
	parsed.Ingredients[1].Component = "glaze" // not a section, salt only used in main steps

	if err := SaveParsedRecipe(ctx, "Chicken", alice.ID, parsed, 0); err != nil {
		t.Fatalf("SaveParsedRecipe: %v", err)
	}

	r := recipeByTitle(t, ctx, alice.ID, "Chicken")
	got := map[string]string{}
	for _, ing := range r.Ingredients {
		got[ing.Name] = ing.Component
	}
	want := map[string]string{"chicken thighs": "main", "salt": "main", "soy sauce": "sauce", "honey": "sauce"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("components = %v, want %v", got, want)
	}
}

func TestSaveParsedRecipeReusesIngredientsCaseInsensitively(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	for _, name := range []string{"Butter", "butter", "BUTTER"} {
		parsed := &RecipeParsed{
			Steps:       map[string][]string{"main": {"Melt the butter."}},
			Ingredients: []Ingredient{{Name: name, Amount: "1 tbsp"}},
		}
		if err := SaveParsedRecipe(ctx, "Recipe with "+name, alice.ID, parsed, 0); err != nil {
			t.Fatalf("SaveParsedRecipe: %v", err)
		}
	}

	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM ingredients`); n != 1 {
		t.Errorf("ingredients rows = %d, want 1 shared butter ingredient", n)
	}
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipe_ingredient`); n != 3 {
		t.Errorf("recipe_ingredient rows = %d, want 3", n)
	}
}

func TestSaveParsedRecipeMarksJobParsed(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	jobID, err := CreateRecipeJob(ctx, models.RecipeJob{Name: "Chicken", Text: "...", Type: "text", User_id: alice.ID})
	if err != nil {
		t.Fatalf("CreateRecipeJob: %v", err)
	}

	if err := SaveParsedRecipe(ctx, "Chicken", alice.ID, sampleParsed(), jobID); err != nil {
		t.Fatalf("SaveParsedRecipe: %v", err)
	}

	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipe_jobs WHERE id = $1 AND parsed`, jobID); n != 1 {
		t.Error("job was not marked parsed")
	}
}

func TestSaveParsedRecipeIsAtomic(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	jobID, err := CreateRecipeJob(ctx, models.RecipeJob{Name: "Broken", Text: "...", Type: "text", User_id: alice.ID})
	if err != nil {
		t.Fatalf("CreateRecipeJob: %v", err)
	}

	// the second ingredient's name is invalid UTF-8, which Postgres rejects partway through the save
	parsed := &RecipeParsed{
		Steps:       map[string][]string{"main": {"Cook."}},
		Ingredients: []Ingredient{{Name: "flour"}, {Name: "bad\xff"}},
	}
	if err := SaveParsedRecipe(ctx, "Broken", alice.ID, parsed, jobID); err == nil {
		t.Fatal("expected SaveParsedRecipe to fail")
	}

	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipes`); n != 0 {
		t.Errorf("recipes = %d, want 0 (rolled back)", n)
	}
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM ingredients`); n != 0 {
		t.Errorf("ingredients = %d, want 0 (rolled back)", n)
	}
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipe_jobs WHERE parsed`); n != 0 {
		t.Error("job was marked parsed even though the save failed")
	}
}

// ---------------------------------------------------------------------------
// reading recipes
// ---------------------------------------------------------------------------

func TestGetAllRecipesVisibility(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	bob := testutil.CreateUser(t, ctx, "bob")
	carol := testutil.CreateUser(t, ctx, "carol")
	testutil.Follow(t, ctx, alice, bob)

	saveSample(t, ctx, alice, "Alice's recipe")
	saveSample(t, ctx, bob, "Bob's recipe")
	saveSample(t, ctx, carol, "Carol's recipe")

	titles := func(userID int) map[string]bool {
		recipes, err := GetAllRecipes(ctx, userID)
		if err != nil {
			t.Fatalf("GetAllRecipes: %v", err)
		}
		got := map[string]bool{}
		for _, r := range recipes {
			got[r.Title] = true
		}
		return got
	}

	if got, want := titles(alice.ID), map[string]bool{"Alice's recipe": true, "Bob's recipe": true}; !reflect.DeepEqual(got, want) {
		t.Errorf("alice sees %v, want own + followed", got)
	}
	if got, want := titles(bob.ID), map[string]bool{"Bob's recipe": true}; !reflect.DeepEqual(got, want) {
		t.Errorf("bob sees %v, want only own (follows aren't mutual)", got)
	}
}

func TestGetAllRecipesEmpty(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	recipes, err := GetAllRecipes(ctx, alice.ID)
	if err != nil {
		t.Fatalf("GetAllRecipes: %v", err)
	}

	// the frontend reads recipes.length, so this must encode as [] not null
	b, _ := json.Marshal(recipes)
	if string(b) != "[]" {
		t.Errorf("no recipes encodes as %s, want []", b)
	}
}

func TestGetAllRecipesRecipeWithoutIngredients(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")

	parsed := &RecipeParsed{Steps: map[string][]string{"main": {"Boil water."}}}
	if err := SaveParsedRecipe(ctx, "Water", alice.ID, parsed, 0); err != nil {
		t.Fatalf("SaveParsedRecipe: %v", err)
	}

	r := recipeByTitle(t, ctx, alice.ID, "Water")
	if len(r.Ingredients) != 0 {
		t.Errorf("ingredients = %+v, want none", r.Ingredients)
	}
}

func TestGetAllRecipesToleratesBadStepIngredients(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")

	if _, err := db.Pool.Exec(ctx, `UPDATE recipes SET step_ingredients = 'not json' WHERE id = $1`, r.ID); err != nil {
		t.Fatal(err)
	}

	r = recipeByTitle(t, ctx, alice.ID, "Chicken")
	if r.StepIngredients != nil {
		t.Errorf("step ingredients = %+v, want nil for bad json", r.StepIngredients)
	}
	if len(r.Ingredients) != 4 {
		t.Errorf("rest of the recipe should still load, got %d ingredients", len(r.Ingredients))
	}
}

// ---------------------------------------------------------------------------
// updating recipes
// ---------------------------------------------------------------------------

func TestUpdateRecipeAmountAndNotes(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")
	salt := r.Ingredients[1]

	err := UpdateRecipe(ctx, r.ID, alice.ID, models.UpdateRecipeRequest{
		UpdatedIngredients: []models.UpdatedIngredient{
			{RecipeIngredientID: salt.RecipeIngredientId, Name: "salt", Amount: "2 tsp", PreparationNotes: "flaky"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateRecipe: %v", err)
	}

	got := recipeByTitle(t, ctx, alice.ID, "Chicken").Ingredients[1]
	if got.Amount != "2 tsp" || got.PreparationNotes != "flaky" || got.RecipeIngredientId != salt.RecipeIngredientId {
		t.Errorf("salt after update = %+v", got)
	}
}

func TestUpdateRecipeRenameKeepsPosition(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")
	chicken := r.Ingredients[0]

	err := UpdateRecipe(ctx, r.ID, alice.ID, models.UpdateRecipeRequest{
		UpdatedIngredients: []models.UpdatedIngredient{
			{RecipeIngredientID: chicken.RecipeIngredientId, Name: "chicken breasts", Amount: "600g", PreparationNotes: "boneless"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateRecipe: %v", err)
	}

	after := recipeByTitle(t, ctx, alice.ID, "Chicken")
	if got := ingredientNames(after.Ingredients); !reflect.DeepEqual(got, []string{"chicken breasts", "salt", "soy sauce", "honey"}) {
		t.Errorf("ingredients after rename = %v, want renamed ingredient still first", got)
	}

	renamed := after.Ingredients[0]
	if renamed.RecipeIngredientId != chicken.RecipeIngredientId {
		t.Errorf("recipe_ingredient id changed from %d to %d; rename must update in place", chicken.RecipeIngredientId, renamed.RecipeIngredientId)
	}
	if renamed.AltAmount != "1 lb" || renamed.Amount != "600g" || renamed.Component != "main" {
		t.Errorf("renamed ingredient = %+v, want alt_amount and component kept", renamed)
	}
	if renamed.IngredientId == chicken.IngredientId {
		t.Error("rename should point at a new ingredient, not rename the shared one")
	}

	// step ingredients follow the rename
	stepIngs := after.StepIngredients["main"]["1"]
	if stepIngs[0].Name != "chicken breasts" {
		t.Errorf("step ingredients not renamed: %+v", stepIngs)
	}
}

func TestUpdateRecipeRenameReusesExistingIngredient(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")
	honey := r.Ingredients[3]
	soy := r.Ingredients[2]

	// rename honey to "Soy Sauce" - should reuse the existing soy sauce ingredient row
	err := UpdateRecipe(ctx, r.ID, alice.ID, models.UpdateRecipeRequest{
		UpdatedIngredients: []models.UpdatedIngredient{
			{RecipeIngredientID: honey.RecipeIngredientId, Name: "Soy Sauce", Amount: "1 tbsp"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateRecipe: %v", err)
	}

	after := recipeByTitle(t, ctx, alice.ID, "Chicken").Ingredients[3]
	if after.IngredientId != soy.IngredientId {
		t.Errorf("ingredient id = %d, want existing soy sauce %d", after.IngredientId, soy.IngredientId)
	}
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM ingredients WHERE LOWER(name) = 'soy sauce'`); n != 1 {
		t.Errorf("soy sauce ingredient rows = %d, want 1", n)
	}
}

func TestUpdateRecipeSteps(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")

	err := UpdateRecipe(ctx, r.ID, alice.ID, models.UpdateRecipeRequest{
		UpdatedSteps: []models.UpdatedStep{
			{StepName: "main", NewSteps: "Season well.\n\n  Roast for 40 minutes.  \n"},
			{StepName: "", NewSteps: "ignored"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateRecipe: %v", err)
	}

	after := recipeByTitle(t, ctx, alice.ID, "Chicken")
	want := map[string][]string{
		"main":  {"Season well.", "Roast for 40 minutes."},
		"sauce": sampleParsed().Steps["sauce"],
	}
	if !reflect.DeepEqual(after.Steps, want) {
		t.Errorf("steps = %q, want %q", after.Steps, want)
	}
	// same number of steps, so the per-step ingredients still line up and are kept
	if !reflect.DeepEqual(after.StepIngredients, sampleParsed().IngredientsUsedForStep) {
		t.Errorf("step ingredients = %+v, want unchanged", after.StepIngredients)
	}
}

func TestUpdateRecipeStepCountChangeClearsStepIngredients(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")

	// main goes from 2 steps to 3, so its step numbers no longer match
	err := UpdateRecipe(ctx, r.ID, alice.ID, models.UpdateRecipeRequest{
		UpdatedSteps: []models.UpdatedStep{{StepName: "main", NewSteps: "Preheat the oven.\nSeason the chicken.\nRoast for 30 minutes."}},
	})
	if err != nil {
		t.Fatalf("UpdateRecipe: %v", err)
	}

	after := recipeByTitle(t, ctx, alice.ID, "Chicken")
	if _, ok := after.StepIngredients["main"]; ok {
		t.Errorf("main step ingredients = %+v, want cleared", after.StepIngredients["main"])
	}
	if !reflect.DeepEqual(after.StepIngredients["sauce"], sampleParsed().IngredientsUsedForStep["sauce"]) {
		t.Errorf("sauce step ingredients = %+v, want untouched", after.StepIngredients["sauce"])
	}
}

func TestUpdateRecipeComponent(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")
	salt := r.Ingredients[1]

	err := UpdateRecipe(ctx, r.ID, alice.ID, models.UpdateRecipeRequest{
		UpdatedIngredients: []models.UpdatedIngredient{
			{RecipeIngredientID: salt.RecipeIngredientId, Name: "salt", Amount: "1 tsp", Component: "sauce"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateRecipe: %v", err)
	}

	if got := recipeByTitle(t, ctx, alice.ID, "Chicken").Ingredients[1].Component; got != "sauce" {
		t.Errorf("component = %q, want sauce", got)
	}
}

func TestUpdateRecipeEmptyComponentKeepsCurrent(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")
	soy := r.Ingredients[2]

	err := UpdateRecipe(ctx, r.ID, alice.ID, models.UpdateRecipeRequest{
		UpdatedIngredients: []models.UpdatedIngredient{
			{RecipeIngredientID: soy.RecipeIngredientId, Name: "soy sauce", Amount: "3 tbsp"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateRecipe: %v", err)
	}

	if got := recipeByTitle(t, ctx, alice.ID, "Chicken").Ingredients[2].Component; got != "sauce" {
		t.Errorf("component = %q, want sauce kept", got)
	}
}

func TestUpdateRecipeInvalidComponentRollsBack(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")

	err := UpdateRecipe(ctx, r.ID, alice.ID, models.UpdateRecipeRequest{
		UpdatedSteps: []models.UpdatedStep{{StepName: "main", NewSteps: "Changed."}},
		UpdatedIngredients: []models.UpdatedIngredient{
			{RecipeIngredientID: r.Ingredients[0].RecipeIngredientId, Name: "chicken thighs", Amount: "1kg"},
			{RecipeIngredientID: r.Ingredients[1].RecipeIngredientId, Name: "salt", Amount: "1 tsp", Component: "frosting"},
		},
	})
	if err == nil {
		t.Fatal("expected an error for a component the recipe doesn't have")
	}

	after := recipeByTitle(t, ctx, alice.ID, "Chicken")
	if !reflect.DeepEqual(after, r) {
		t.Errorf("recipe changed despite the failed update:\nbefore %+v\nafter  %+v", r, after)
	}
}

func TestUpdateRecipeLegacyNullColumns(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")
	salt := r.Ingredients[1]

	// rows created by the old rename flow have NULL amount/prep_notes/alt_amount
	_, err := db.Pool.Exec(ctx,
		`UPDATE recipe_ingredient SET amount = NULL, prep_notes = NULL, alt_amount = NULL WHERE id = $1`,
		salt.RecipeIngredientId)
	if err != nil {
		t.Fatal(err)
	}

	err = UpdateRecipe(ctx, r.ID, alice.ID, models.UpdateRecipeRequest{
		UpdatedIngredients: []models.UpdatedIngredient{
			{RecipeIngredientID: salt.RecipeIngredientId, Name: "salt", Amount: "a pinch"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateRecipe on a row with NULL columns: %v", err)
	}
	if got := recipeByTitle(t, ctx, alice.ID, "Chicken").Ingredients[1].Amount; got != "a pinch" {
		t.Errorf("amount = %q, want a pinch", got)
	}
}

func TestUpdateRecipeOtherUsersRecipe(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	mallory := testutil.CreateUser(t, ctx, "mallory")
	testutil.Follow(t, ctx, mallory, alice)
	r := saveSample(t, ctx, alice, "Chicken")

	err := UpdateRecipe(ctx, r.ID, mallory.ID, models.UpdateRecipeRequest{
		UpdatedSteps: []models.UpdatedStep{{StepName: "main", NewSteps: "Hacked."}},
	})
	if !errors.Is(err, ErrRecipeNotFound) {
		t.Fatalf("error = %v, want ErrRecipeNotFound", err)
	}

	if after := recipeByTitle(t, ctx, alice.ID, "Chicken"); !reflect.DeepEqual(after, r) {
		t.Error("another user was able to change the recipe")
	}
}

func TestUpdateRecipeIngredientFromAnotherRecipe(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	mine := saveSample(t, ctx, alice, "Mine")
	other := saveSample(t, ctx, alice, "Other")

	err := UpdateRecipe(ctx, mine.ID, alice.ID, models.UpdateRecipeRequest{
		UpdatedIngredients: []models.UpdatedIngredient{
			{RecipeIngredientID: other.Ingredients[0].RecipeIngredientId, Name: "tofu", Amount: "1 block"},
		},
	})
	if err == nil {
		t.Fatal("expected an error editing an ingredient row that belongs to another recipe")
	}

	if after := recipeByTitle(t, ctx, alice.ID, "Other"); !reflect.DeepEqual(after, other) {
		t.Error("the other recipe was changed")
	}
}

// ---------------------------------------------------------------------------
// deleting recipes
// ---------------------------------------------------------------------------

func TestDeleteRecipe(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")

	if err := DeleteRecipe(ctx, r.ID, alice.ID); err != nil {
		t.Fatalf("DeleteRecipe: %v", err)
	}

	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipes`); n != 0 {
		t.Errorf("recipes = %d, want 0", n)
	}
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipe_ingredient`); n != 0 {
		t.Errorf("recipe_ingredient = %d, want 0 (cascade)", n)
	}
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM ingredients`); n != 4 {
		t.Errorf("ingredients = %d, want 4 (shared ingredients are kept)", n)
	}
}

func TestDeleteRecipeOnlyOwner(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	mallory := testutil.CreateUser(t, ctx, "mallory")
	r := saveSample(t, ctx, alice, "Chicken")

	if err := DeleteRecipe(ctx, r.ID, mallory.ID); err != nil {
		t.Fatalf("DeleteRecipe: %v", err)
	}

	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipes`); n != 1 {
		t.Error("a user deleted someone else's recipe")
	}
}

// ---------------------------------------------------------------------------
// ingredients
// ---------------------------------------------------------------------------

func TestGetAllIngredients(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	saveSample(t, ctx, alice, "Chicken")

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO grocery_tag (ingredient_id, category, location)
		SELECT id, 'seasoning', 'pantry' FROM ingredients WHERE name = 'salt'`)
	if err != nil {
		t.Fatal(err)
	}

	ingredients, err := GetAllIngredients(ctx)
	if err != nil {
		t.Fatalf("GetAllIngredients: %v", err)
	}

	var names []string
	for _, ing := range ingredients {
		names = append(names, ing.Name)
		if ing.Name == "salt" {
			if ing.Category == nil || *ing.Category != "seasoning" || ing.Location == nil || *ing.Location != "pantry" {
				t.Errorf("salt tags = %v / %v", ing.Category, ing.Location)
			}
		} else if ing.Category != nil || ing.Location != nil {
			t.Errorf("%s should have no tags", ing.Name)
		}
	}
	if want := []string{"chicken thighs", "honey", "salt", "soy sauce"}; !reflect.DeepEqual(names, want) {
		t.Errorf("ingredients = %v, want sorted %v", names, want)
	}
}

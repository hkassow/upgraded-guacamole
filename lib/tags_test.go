package lib

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"go-guacamole/internal/testutil"
	"go-guacamole/models"
)

func TestNormalizeTags(t *testing.T) {
	got, err := NormalizeTags([]string{"  Vegan ", "#vegan", "Quick   Dinner", "", "  ", "baking", "VEGAN"})
	if err != nil {
		t.Fatalf("NormalizeTags: %v", err)
	}
	if want := []string{"vegan", "quick dinner", "baking"}; !reflect.DeepEqual(got, want) {
		t.Errorf("NormalizeTags = %q, want %q", got, want)
	}

	if _, err := NormalizeTags([]string{strings.Repeat("a", maxTagLength+1)}); !errors.Is(err, ErrInvalidTags) {
		t.Errorf("too long tag: err = %v, want ErrInvalidTags", err)
	}
	// length is counted in characters, not bytes
	if _, err := NormalizeTags([]string{strings.Repeat("é", maxTagLength)}); err != nil {
		t.Errorf("%d accented characters should be allowed: %v", maxTagLength, err)
	}

	tooMany := make([]string, maxTagsPerRecipe+1)
	for i := range tooMany {
		tooMany[i] = strings.Repeat("x", i+1)
	}
	if _, err := NormalizeTags(tooMany); !errors.Is(err, ErrInvalidTags) {
		t.Errorf("too many tags: err = %v, want ErrInvalidTags", err)
	}
}

// setTags edits a recipe's tags through UpdateRecipe, the same way the edit form does.
func setTags(t *testing.T, recipeID, userID int, tags ...string) error {
	t.Helper()
	if tags == nil {
		tags = []string{} // an explicit empty list clears tags; nil would mean "unchanged"
	}
	return UpdateRecipe(context.Background(), recipeID, userID, models.UpdateRecipeRequest{Tags: &tags})
}

func TestRecipeTags(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")

	if len(r.Tags) != 0 || r.Tags == nil {
		t.Fatalf("new recipe tags = %#v, want empty (non-nil) list", r.Tags)
	}

	if err := setTags(t, r.ID, alice.ID, "Dinner", "quick", "dinner"); err != nil {
		t.Fatalf("setting tags: %v", err)
	}
	if got := recipeByTitle(t, ctx, alice.ID, "Chicken").Tags; !reflect.DeepEqual(got, []string{"dinner", "quick"}) {
		t.Errorf("tags = %q, want sorted, normalized, de-duplicated", got)
	}

	// replacing keeps what's still there and drops the rest
	if err := setTags(t, r.ID, alice.ID, "quick", "spicy"); err != nil {
		t.Fatal(err)
	}
	if got := recipeByTitle(t, ctx, alice.ID, "Chicken").Tags; !reflect.DeepEqual(got, []string{"quick", "spicy"}) {
		t.Errorf("tags after replace = %q", got)
	}

	// an edit that doesn't send tags leaves them alone
	err := UpdateRecipe(ctx, r.ID, alice.ID, models.UpdateRecipeRequest{
		UpdatedSteps: []models.UpdatedStep{{StepName: "main", NewSteps: "Season.\nRoast."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := recipeByTitle(t, ctx, alice.ID, "Chicken").Tags; !reflect.DeepEqual(got, []string{"quick", "spicy"}) {
		t.Errorf("tags after an edit without tags = %q, want unchanged", got)
	}

	// an empty list removes them all
	if err := setTags(t, r.ID, alice.ID); err != nil {
		t.Fatal(err)
	}
	if got := recipeByTitle(t, ctx, alice.ID, "Chicken").Tags; len(got) != 0 {
		t.Errorf("tags after clearing = %q", got)
	}
}

func TestRecipeTagsAreShared(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	bob := testutil.CreateUser(t, ctx, "bob")
	a := saveSample(t, ctx, alice, "Alice's")
	b := saveSample(t, ctx, bob, "Bob's")

	if err := setTags(t, a.ID, alice.ID, "Vegan"); err != nil {
		t.Fatal(err)
	}
	if err := setTags(t, b.ID, bob.ID, "vegan"); err != nil {
		t.Fatal(err)
	}

	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM tags`); n != 1 {
		t.Errorf("tags rows = %d, want one shared vegan tag", n)
	}
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipe_tags`); n != 2 {
		t.Errorf("recipe_tags rows = %d, want 2", n)
	}
}

func TestRecipeTagsOnlyOwner(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	mallory := testutil.CreateUser(t, ctx, "mallory")
	r := saveSample(t, ctx, alice, "Chicken")

	if err := setTags(t, r.ID, mallory.ID, "gross"); !errors.Is(err, ErrRecipeNotFound) {
		t.Errorf("err = %v, want ErrRecipeNotFound", err)
	}
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipe_tags`); n != 0 {
		t.Error("another user tagged the recipe")
	}
}

func TestRecipeTagsInvalidRollsBack(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")
	if err := setTags(t, r.ID, alice.ID, "dinner"); err != nil {
		t.Fatal(err)
	}

	tags := []string{"lunch", strings.Repeat("a", maxTagLength+1)}
	err := UpdateRecipe(ctx, r.ID, alice.ID, models.UpdateRecipeRequest{
		Tags:         &tags,
		UpdatedSteps: []models.UpdatedStep{{StepName: "main", NewSteps: "Changed."}},
	})
	if !errors.Is(err, ErrInvalidTags) {
		t.Fatalf("err = %v, want ErrInvalidTags", err)
	}

	after := recipeByTitle(t, ctx, alice.ID, "Chicken")
	if !reflect.DeepEqual(after.Tags, []string{"dinner"}) || after.Steps["main"][0] != r.Steps["main"][0] {
		t.Errorf("recipe changed despite the invalid tags: tags %q, steps %q", after.Tags, after.Steps["main"])
	}
}

func TestDeleteRecipeRemovesTags(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	r := saveSample(t, ctx, alice, "Chicken")
	if err := setTags(t, r.ID, alice.ID, "dinner"); err != nil {
		t.Fatal(err)
	}

	if err := DeleteRecipe(ctx, r.ID, alice.ID); err != nil {
		t.Fatal(err)
	}
	if n := testutil.Count(t, ctx, `SELECT COUNT(*) FROM recipe_tags`); n != 0 {
		t.Errorf("recipe_tags rows = %d, want 0 (cascade)", n)
	}
}

func TestGetAllRecipesOwner(t *testing.T) {
	ctx := testutil.SetupDB(t)
	alice := testutil.CreateUser(t, ctx, "alice")
	bob := testutil.CreateUser(t, ctx, "bob")
	testutil.Follow(t, ctx, alice, bob)
	saveSample(t, ctx, bob, "Bob's")

	r := recipeByTitle(t, ctx, alice.ID, "Bob's")
	if r.OwnerID != bob.ID || r.OwnerName != "bob" {
		t.Errorf("owner = %d %q, want bob", r.OwnerID, r.OwnerName)
	}
}

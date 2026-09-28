package lib

import (
	"log"
	"context"
	"fmt"
	"errors"
	"strings"
	"encoding/json"
	"github.com/jackc/pgx/v5"

	"go-guacamole/db"
	"go-guacamole/models"
)

type ParsedIngredient struct {
    Name             	string `json:"name"`
    Amount           	string `json:"amount"`
    AltAmount           string `json:"alt_amount"`
    PreparationNotes 	string `json:"preparation_notes"`
    Component           string `json:"component"`
    IngredientId     	int `json:"ingredient_id"`
    RecipeIngredientId	int `json:"recipe_ingredient_id"`
}

type RecipeResponse struct {
    ID          int                 `json:"id"`
    Title       string              `json:"title"`
    Steps       map[string][]string `json:"steps"`
    StepIngredients map[string]map[string][]StepIngredient `json:"step_ingredients"`
    Ingredients []ParsedIngredient  `json:"ingredients"`
    Tags        []string            `json:"tags"`
    OwnerName   string              `json:"owner_name"`
    OwnerID     int                 `json:"-"`
    IsMine      bool                `json:"is_mine"`
}

func HandleManualRecipePost(ctx context.Context, userID int, rawRecipe models.RawRecipe) error {
    steps := map[string][]string{
        "main": cleanStepLines(rawRecipe.Text),
    }

    ingredients := make([]Ingredient, 0, len(rawRecipe.Ingredients))
    for _, ingredient := range rawRecipe.Ingredients {
        ingredients = append(ingredients, Ingredient{
            Name:              ingredient.Name,
            Amount:            ingredient.Amount,
            PreparationNotes:  ingredient.PreparationNotes,
        })
    }

    parsed := RecipeParsed{
        Steps: steps,
        Ingredients: ingredients,
    }
    log.Printf("Manually parsed recipe being added:  %+v", parsed)

    return SaveParsedRecipe(ctx, rawRecipe.Name, userID, &parsed, 0)
}

// SaveParsedRecipe saves the recipe and its ingredients. If jobID is non-zero the recipe_job is
// marked parsed in the same transaction, so a job can't be saved twice.
func SaveParsedRecipe(ctx context.Context, title string,  userID int, parsed *RecipeParsed, jobID int) error {
	pool := db.Pool

	// covers model output, reused parsed_json from older runs, and manual recipes (all "main")
	if dropped := keepKnownStepIngredients(parsed.IngredientsUsedForStep, ingredientNamesOf(parsed.Ingredients)); dropped > 0 {
		log.Printf("recipe %q: dropped %d step ingredients that aren't in the ingredient list", title, dropped)
	}
	normalizeIngredientComponents(parsed)

	steps, err := json.Marshal(parsed.Steps)
    if err != nil {
        return err
    }

    var stepIngredients *string
	if len(parsed.IngredientsUsedForStep) > 0 {
		b, mErr := json.Marshal(parsed.IngredientsUsedForStep)
		if mErr != nil {
			return mErr
		}
		s := string(b)
		stepIngredients = &s
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // no-op once Commit succeeds

	var recipeID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO recipes (title, steps, step_ingredients, user_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		title, 
		steps,
        stepIngredients,
        userID,
	).Scan(&recipeID)
	if err != nil {
		return err
	}

	for _, ing := range parsed.Ingredients {
        	var ingredientID int64

        	// Try to find ingredient
        	err = tx.QueryRow(ctx,
        	    `SELECT id FROM ingredients WHERE LOWER(name) = LOWER($1)`,
        	    ing.Name,
        	).Scan(&ingredientID)

     	    if errors.Is(err, pgx.ErrNoRows) { // not found → insert
        	    err = tx.QueryRow(ctx,
        	        `INSERT INTO ingredients (name)
        	         VALUES ($1)
        	         RETURNING id`,
        	        ing.Name,
        	    ).Scan(&ingredientID)
        	    if err != nil {
        	        return err
        	    }
        	} else if err != nil {
        	    return fmt.Errorf("fetch ingredient %q: %w", ing.Name, err)
        	}

        	// Link recipe + ingredient
        	_, err = tx.Exec(ctx,
        	    `INSERT INTO recipe_ingredient (recipe_id, ingredient_id, amount, alt_amount, prep_notes, component)
        	     VALUES ($1, $2, $3, $4, $5, $6)`,
        	    recipeID, ingredientID, ing.Amount, ing.AltAmount, ing.PreparationNotes, ing.Component,
        	)
        	if err != nil {
        	    return err
        	}
    	}

	if jobID != 0 {
		_, err = tx.Exec(ctx,
			`UPDATE recipe_jobs SET parsed = TRUE, updated_at = NOW() WHERE id = $1`,
			jobID,
		)
		if err != nil {
			return fmt.Errorf("mark recipe_job %d parsed: %w", jobID, err)
		}
	}

	return tx.Commit(ctx)
}


func GetAllRecipes(ctx context.Context, userID int) ([]RecipeResponse, error) {
    rows, err := db.Pool.Query(ctx, `
        SELECT r.id, r.title, r.steps, r.step_ingredients,
            COALESCE(json_agg(json_build_object(
                'name', i.name, 
                'amount', ri.amount,
                'alt_amount', ri.alt_amount,
                'preparation_notes', ri.prep_notes,
                'component', ri.component,
	            'ingredient_id', i.id,
	            'recipe_ingredient_id', ri.id
            ) ORDER BY ri.id) FILTER (WHERE ri.id IS NOT NULL), '[]') as ingredients,
            ARRAY(
                SELECT t.name FROM recipe_tags rt JOIN tags t ON t.id = rt.tag_id
                WHERE rt.recipe_id = r.id ORDER BY t.name
            ) as tags,
            COALESCE(r.user_id, 0), COALESCE(u.display_name, '')
        FROM recipes r
        LEFT JOIN recipe_ingredient ri ON r.id = ri.recipe_id
	    LEFT JOIN ingredients i on ri.ingredient_id = i.id
        LEFT JOIN users u ON u.id = r.user_id
        WHERE r.user_id = $1 or r.user_id in (SELECT followee_id FROM users_follows WHERE follower_id = $1)
        GROUP BY r.id, u.id
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    // empty slice (not nil) so a user with no recipes gets [] rather than null
    recipes := []RecipeResponse{}
    for rows.Next() {
        var r RecipeResponse
        var stepsBytes []byte
        var stepIngredientsStr *string
        var ingredientsBytes []byte

        if err := rows.Scan(&r.ID, &r.Title, &stepsBytes, &stepIngredientsStr, &ingredientsBytes,
            &r.Tags, &r.OwnerID, &r.OwnerName); err != nil {
            return nil, err
        }
        if r.Tags == nil {
            r.Tags = []string{} // [] not null in the JSON
        }

        if err := json.Unmarshal(stepsBytes, &r.Steps); err != nil {
            return nil, err
        }

        if err := json.Unmarshal(ingredientsBytes, &r.Ingredients); err != nil {
            return nil, err
        }

        // dont fail the return if stepIngredient fails
        if stepIngredientsStr != nil {
            if err := json.Unmarshal([]byte(*stepIngredientsStr), &r.StepIngredients); err != nil {
                log.Printf("recipe %d: bad step_ingredients json: %v", r.ID, err)
                r.StepIngredients = nil
            }
        }

        // recipes saved before step ingredients were filtered on save can still list things
        // like "dough"; only show ingredients that are actually in the recipe
        names := make([]string, len(r.Ingredients))
        for i, ing := range r.Ingredients {
            names[i] = ing.Name
        }
        keepKnownStepIngredients(r.StepIngredients, names)
        if len(r.StepIngredients) == 0 {
            r.StepIngredients = nil
        }

        recipes = append(recipes, r)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }

    return recipes, nil
}

// ErrRecipeNotFound is returned when a recipe doesn't exist or doesn't belong to the user.
var ErrRecipeNotFound = errors.New("recipe not found")

// UpdateRecipe applies step and ingredient edits to a recipe owned by userID.
func UpdateRecipe(ctx context.Context, recipeID int, userID int, req models.UpdateRecipeRequest) error {
    tx, err := db.Pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx) // no-op once Commit succeeds

    var stepsJSON string
    var stepIngredientsStr *string
    err = tx.QueryRow(ctx,
        `SELECT steps, step_ingredients FROM recipes WHERE id = $1 AND user_id = $2 FOR UPDATE`,
        recipeID, userID,
    ).Scan(&stepsJSON, &stepIngredientsStr)

    if errors.Is(err, pgx.ErrNoRows) {
        return ErrRecipeNotFound
    }
    if err != nil {
        return fmt.Errorf("failed to load recipe: %w", err)
    }

    // nil means the tags weren't edited; an empty list removes them all
    if req.Tags != nil {
        tags, err := NormalizeTags(*req.Tags)
        if err != nil {
            return err
        }
        if err := setRecipeTags(ctx, tx, recipeID, tags); err != nil {
            return err
        }
    }

    var steps map[string][]string
    if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
        return fmt.Errorf("invalid steps json: %w", err)
    }

    var stepIngredients map[string]map[string][]StepIngredient
    if stepIngredientsStr != nil {
        if err := json.Unmarshal([]byte(*stepIngredientsStr), &stepIngredients); err != nil {
            log.Printf("recipe %d: bad step_ingredients json, leaving untouched: %v", recipeID, err)
            stepIngredients = nil
        }
    }
    stepIngredientsChanged := false

    for _, stepUpdate := range req.UpdatedSteps {
        key := stepUpdate.StepName
        if key == "" {
            continue
        }

        newStepLines := cleanStepLines(stepUpdate.NewSteps)

        // step ingredients are keyed by step number; if steps were added or removed the numbers
        // no longer line up, so drop them rather than show ingredients under the wrong step
        if _, ok := stepIngredients[key]; ok && len(newStepLines) != len(steps[key]) {
            delete(stepIngredients, key)
            stepIngredientsChanged = true
        }

        steps[key] = newStepLines
    }

    for _, ing := range req.UpdatedIngredients {

        // 1. Get the current ingredient info to detect what changed
        var currentName, currentAmount, currentNotes, currentComponent string

        err := tx.QueryRow(ctx,
            `SELECT i.name, COALESCE(ri.amount, ''), COALESCE(ri.prep_notes, ''), ri.component
             FROM recipe_ingredient ri
             JOIN ingredients i ON ri.ingredient_id = i.id
             WHERE ri.id = $1 AND ri.recipe_id = $2`,
            ing.RecipeIngredientID, recipeID,
        ).Scan(&currentName, &currentAmount, &currentNotes, &currentComponent)
        if err != nil {
            return fmt.Errorf("fetch existing recipe ingredient %d: %w", ing.RecipeIngredientID, err)
        }

        component := ing.Component
        if component == "" {
            component = currentComponent
        } else if _, ok := steps[component]; !ok && component != currentComponent && component != defaultComponent {
            return fmt.Errorf("ingredient %q: recipe has no %q section", ing.Name, component)
        }

        // --------------------------------
        // Case A: Only amount, notes or component changed
        // --------------------------------
        if ing.Name == currentName {
            if ing.Amount != currentAmount || ing.PreparationNotes != currentNotes || component != currentComponent {
                _, err := tx.Exec(ctx,
                    `UPDATE recipe_ingredient
                     SET amount = $1, prep_notes = $2, component = $3
                     WHERE id = $4`,
                    ing.Amount, ing.PreparationNotes, component, ing.RecipeIngredientID,
                )
                if err != nil {
                    return fmt.Errorf("update recipe_ingredient: %w", err)
                }
            }
            continue
        }

        // --------------------------------
        // Case B: Ingredient NAME changed → re-point the existing row
        // --------------------------------

        // 1. Find or create the ingredient with the new name
        var newIngredientID int
        err = tx.QueryRow(ctx,
            `SELECT id FROM ingredients WHERE LOWER(name) = LOWER($1)`,
            ing.Name,
        ).Scan(&newIngredientID)

        if errors.Is(err, pgx.ErrNoRows) {
            err = tx.QueryRow(ctx,
                `INSERT INTO ingredients (name)
                 VALUES ($1) RETURNING id`,
                ing.Name,
            ).Scan(&newIngredientID)
            if err != nil {
                return fmt.Errorf("create new ingredient: %w", err)
            }
        } else if err != nil {
            return fmt.Errorf("fetch ingredient: %w", err)
        }

        // 2. Update the existing row in place: keeps its id (ordering) and alt_amount.
        _, err = tx.Exec(ctx,
            `UPDATE recipe_ingredient
             SET ingredient_id = $1, amount = $2, prep_notes = $3, component = $4
             WHERE id = $5 AND recipe_id = $6`,
            newIngredientID, ing.Amount, ing.PreparationNotes, component, ing.RecipeIngredientID, recipeID,
        )
        if err != nil {
            return fmt.Errorf("update recipe_ingredient: %w", err)
        }

        // 3. Keep step_ingredients pointing at the renamed ingredient
        if renameStepIngredient(stepIngredients, currentName, ing.Name) {
            stepIngredientsChanged = true
        }
    }

    updatedStepsJSON, err := json.Marshal(steps)
    if err != nil {
        return fmt.Errorf("failed to marshal steps json: %w", err)
    }

    if stepIngredientsChanged {
        b, err := json.Marshal(stepIngredients)
        if err != nil {
            return fmt.Errorf("failed to marshal step_ingredients json: %w", err)
        }
        _, err = tx.Exec(ctx,
            `UPDATE recipes SET steps = $1, step_ingredients = $2 WHERE id = $3`,
            string(updatedStepsJSON), string(b), recipeID,
        )
    } else {
        _, err = tx.Exec(ctx,
            `UPDATE recipes SET steps = $1 WHERE id = $2`,
            string(updatedStepsJSON), recipeID,
        )
    }
    if err != nil {
        return fmt.Errorf("failed updating recipe steps: %w", err)
    }

    return tx.Commit(ctx)
}

// renameStepIngredient renames every step_ingredients entry matching oldName
// (case-insensitive). Returns true if anything changed.
func renameStepIngredient(stepIngredients map[string]map[string][]StepIngredient, oldName, newName string) bool {
    changed := false
    old := strings.ToLower(strings.TrimSpace(oldName))
    for _, byStep := range stepIngredients {
        for _, used := range byStep {
            for i := range used {
                if strings.ToLower(strings.TrimSpace(used[i].Name)) == old {
                    used[i].Name = newName
                    changed = true
                }
            }
        }
    }
    return changed
}

func DeleteRecipe(ctx context.Context, recipeID int, userID int) error {
    _, err := db.Pool.Exec(ctx,
        `DELETE FROM recipes WHERE id = $1 and user_id = $2`,
        recipeID, userID,
    )
    if err != nil {
        return fmt.Errorf("error deleting recipe: %w", err)
    }

    return nil
}

func CreateRecipeJob(ctx context.Context, job models.RecipeJob) (int, error) {
    imagesJSON, _ := json.Marshal(job.Images)

    var jobID int
    err := db.Pool.QueryRow(ctx,
        `INSERT INTO recipe_jobs (title, text, images, type, parsed, user_id)
         VALUES ($1, $2, $3, $4, FALSE, $5)
         RETURNING id`,
        job.Name, job.Text, imagesJSON, job.Type, job.User_id,
    ).Scan(&jobID)

    if err != nil {
        log.Println("Failed to create recipe_job:", err)
        return 0, err
    }

    return jobID, nil
}

func SaveRecipeJobParsedJSON(ctx context.Context, jobID int, parsedJSON []byte) error {
    _, err := db.Pool.Exec(ctx,
        `UPDATE recipe_jobs
         SET parsed_json = $1, updated_at = NOW()
         WHERE id = $2`,
        parsedJSON, jobID,
    )

    if err != nil {
        log.Println("Failed to save recipe_job parsed json:", err)
    }

    return err
}

func SaveRecipeJobTranscript(ctx context.Context, jobID int, transcript string) error {
    _, err := db.Pool.Exec(ctx,
        `UPDATE recipe_jobs
         SET transcript = $1, updated_at = NOW()
         WHERE id = $2`,
        transcript, jobID,
    )

    if err != nil {
        log.Println("Failed to save recipe_job transcript:", err)
    }

    return err
}

func IncrementRecipeJobFailCount(ctx context.Context, jobID int) error {
    _, err := db.Pool.Exec(ctx,
        `UPDATE recipe_jobs
         SET fail_count = fail_count + 1, updated_at = NOW()
         WHERE id = $1`,
        jobID,
    )

    if err != nil {
        log.Println("Failed to increment recipe_job fail_count:", err)
    }

    return err
}

func LoadUnparsedRecipeJobs(ctx context.Context) (error) {
    rows, err := db.Pool.Query(ctx,
        `SELECT id, title, text, images, type, user_id, parsed_json, COALESCE(transcript, '')
         FROM recipe_jobs
         WHERE parsed = FALSE AND fail_count < $1`,
        MaxRecipeJobFailures,
    )
    if err != nil {
        return err
    }
    defer rows.Close()

    count := 0

    for rows.Next() {
        var job models.RecipeJob
        var imagesJSON []byte
	    if err := rows.Scan(&job.ID, &job.Name, &job.Text, &imagesJSON, &job.Type, &job.User_id, &job.ParsedJSON, &job.Transcript); err != nil {
            return err
        }
        if len(imagesJSON) > 0 {
            json.Unmarshal(imagesJSON, &job.Images)
        }

        // Queue the job directly
        RecipeQueue <- job
        count++
    }

    log.Printf("Queued %d unparsed recipe jobs\n", count)
    return nil
}

func cleanStepLines(text string) []string {
    raw := strings.Split(text, "\n")
    cleaned := make([]string, 0, len(raw))

    for _, line := range raw {
        trimmed := strings.TrimSpace(line)
        if trimmed != "" {
            cleaned = append(cleaned, trimmed)
        }
    }

    return cleaned
}

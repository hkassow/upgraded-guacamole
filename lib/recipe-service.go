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
    PreparationNotes 	string `json:"preparation_notes"`
    IngredientId     	int `json:"ingredient_id"`
    RecipeIngredientId	int `json:"recipe_ingredient_id"`
}

type RecipeResponse struct {
    ID          int                 `json:"id"`
    Title       string              `json:"title"`
    Steps       map[string][]string `json:"steps"`      
    Ingredients []ParsedIngredient  `json:"ingredients"`
}

func SaveParsedRecipe(ctx context.Context, title string,  userID int, parsed *RecipeParsed) error {
	pool := db.Pool

	steps, err := json.Marshal(parsed.Steps)
        if err != nil {
                return err
        }

	var recipeID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO recipes (title, steps, user_id) VALUES ($1, $2, $3) RETURNING id`,
		title, 
		steps,
        userID,
	).Scan(&recipeID)
	if err != nil {
		return err
	}

	for _, ing := range parsed.Ingredients {
        	var ingredientID int64

        	// Try to find ingredient
        	err = pool.QueryRow(ctx,
        	    `SELECT id FROM ingredients WHERE LOWER(name) = LOWER($1)`,
        	    ing.Name,
        	).Scan(&ingredientID)

        	if err != nil { // not found → insert
        	    err = pool.QueryRow(ctx,
        	        `INSERT INTO ingredients (name)
        	         VALUES ($1)
        	         RETURNING id`,
        	        ing.Name,
        	    ).Scan(&ingredientID)
        	    if err != nil {
        	        return err
        	    }
        	}

        	// Link recipe + ingredient
        	_, err = pool.Exec(ctx,
        	    `INSERT INTO recipe_ingredient (recipe_id, ingredient_id, amount, prep_notes)
        	     VALUES ($1, $2, $3, $4)`,
        	    recipeID, ingredientID, ing.Amount, ing.PreparationNotes,
        	)
        	if err != nil {
        	    return err
        	}
    	}

	return nil
}


func GetAllRecipes(ctx context.Context, userID int) ([]RecipeResponse, error) {
    rows, err := db.Pool.Query(ctx, `
        SELECT r.id, r.title, r.steps, 
            json_agg(json_build_object(
                'name', i.name, 
               'amount', ri.amount, 
               'preparation_notes', ri.prep_notes,
	            'ingredient_id', i.id,
	            'recipe_ingredient_id', ri.id
            )) as ingredients
        FROM recipes r
        LEFT JOIN recipe_ingredient ri ON r.id = ri.recipe_id
	    LEFT JOIN ingredients i on ri.ingredient_id = i.id
        WHERE r.user_id = $1 or r.user_id in (SELECT followee_id FROM users_follows WHERE follower_id = $1)
        GROUP BY r.id
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var recipes []RecipeResponse
    for rows.Next() {
        var r RecipeResponse
        var stepsBytes []byte
        var ingredientsBytes []byte

        if err := rows.Scan(&r.ID, &r.Title, &stepsBytes, &ingredientsBytes); err != nil {
            return nil, err
        }

        if err := json.Unmarshal(stepsBytes, &r.Steps); err != nil {
            return nil, err
        }
        if err := json.Unmarshal(ingredientsBytes, &r.Ingredients); err != nil {
            return nil, err
        }

        recipes = append(recipes, r)
    }

    return recipes, nil
}

func UpdateRecipe(ctx context.Context, recipeID int, req models.UpdateRecipeRequest) error {
    var stepsJSON string
    err := db.Pool.QueryRow(ctx,
        `SELECT steps FROM recipes WHERE id = $1`,
        recipeID,
    ).Scan(&stepsJSON)

    if err != nil {
        return fmt.Errorf("failed to load recipe: %w", err)
    }


    var steps map[string][]string
    if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
        return fmt.Errorf("invalid steps json: %w", err)
    }

    for _, stepUpdate := range req.UpdatedSteps {
    	key := stepUpdate.StepName
	if key == "" {
		continue
	}

	newStepLines := cleanStepLines(stepUpdate.NewSteps)

	steps[key] = newStepLines
    }

    updatedStepsJSON, err := json.Marshal(steps)
    if err != nil {
    	return fmt.Errorf("failed to marshal steps json: %w", err)
    }

    _, err = db.Pool.Exec(ctx,
    	`UPDATE recipes SET steps = $1 WHERE id = $2`,
    	string(updatedStepsJSON), recipeID,
    )

    if err != nil {
    	return fmt.Errorf("failed updating recipe steps: %w", err)
    }


    for _, ing := range req.UpdatedIngredients {

        // 1. Get the current ingredient info to detect what changed
        var currentName, currentAmount, currentNotes string

        err := db.Pool.QueryRow(ctx,
            `SELECT i.name, ri.amount, ri.prep_notes
             FROM recipe_ingredient ri
             JOIN ingredients i ON ri.ingredient_id = i.id
             WHERE ri.id = $1`,
            ing.RecipeIngredientID,
        ).Scan(&currentName, &currentAmount, &currentNotes)
        if err != nil {
            return fmt.Errorf("fetch existing recipe ingredient: %w", err)
        }

        // --------------------------------
        // Case A: Only amount or notes changed
        // --------------------------------
        if ing.Name == currentName {
            if ing.Amount != currentAmount || ing.PreparationNotes != currentNotes {
                _, err := db.Pool.Exec(ctx,
                    `UPDATE recipe_ingredient
                     SET amount = $1, prep_notes = $2
                     WHERE id = $3`,
                    ing.Amount, ing.PreparationNotes, ing.RecipeIngredientID,
                )
                if err != nil {
                    return fmt.Errorf("update recipe_ingredient: %w", err)
                }
		        log.Println("Updating recipe ingredient:", ing.Name)
            }
            continue
        }

        // --------------------------------
        // Case B: Ingredient NAME changed → full replace logic
        // --------------------------------

        // 1. Delete old recipe_ingredient row
	    log.Println("NEW NAME DELETING RECIPE INGRED", ing.Name)
        _, err = db.Pool.Exec(ctx,
            `DELETE FROM recipe_ingredient WHERE id = $1`,
            ing.RecipeIngredientID,
        )
        if err != nil {
            return fmt.Errorf("delete old recipe_ingredient: %w", err)
        }

        // 2. Check if ingredient with new name already exists
        var newIngredientID int
        err = db.Pool.QueryRow(ctx,
            `SELECT id FROM ingredients WHERE LOWER(name) = LOWER($1)`,
            ing.Name,
        ).Scan(&newIngredientID)

	    log.Println("DOES INGREDIENT ALREADY EXIST:", newIngredientID)
	    if err != nil {
            if errors.Is(err, pgx.ErrNoRows) {
		        log.Println("INGREDIENT DOESNT EXIST CREATING IT")
                // Create new ingredient
                err = db.Pool.QueryRow(ctx,
                    `INSERT INTO ingredients (name)
                    VALUES ($1) RETURNING id`,
                    ing.Name,
                ).Scan(&newIngredientID)

                if err != nil {
                    return fmt.Errorf("create new ingredient: %w", err)
                }
            } else {
                return fmt.Errorf("fetch ingredient: %w", err)
            }
        }

        // 4. Create new recipe_ingredient linking recipe + new ingredient
        _, err = db.Pool.Exec(ctx,
            `INSERT INTO recipe_ingredient
                (recipe_id, ingredient_id, amount, prep_notes)
             VALUES ($1, $2, $3, $4)`,
            recipeID, newIngredientID, ing.Amount, ing.PreparationNotes,
        )
        if err != nil {
            return fmt.Errorf("create new recipe_ingredient: %w", err)
        }
	    log.Println("DONE")
    }

    return nil
}

func CreateRecipeJob(ctx context.Context, name, text string, user_id int) (int, error) {
    var jobID int
    err := db.Pool.QueryRow(ctx,
        `INSERT INTO recipe_jobs (title, text, parsed, user_id)
         VALUES ($1, $2, FALSE, $3)
         RETURNING id`,
        name, text, user_id,
    ).Scan(&jobID)

    if err != nil {
        log.Println("Failed to create recipe_job:", err)
        return 0, err
    }

    return jobID, nil
}

func MarkRecipeJobParsed(ctx context.Context, jobID int) error {
    _, err := db.Pool.Exec(ctx,
        `UPDATE recipe_jobs
         SET parsed = TRUE
         WHERE id = $1`,
        jobID,
    )

    if err != nil {
        log.Println("Failed to update recipe_job:", err)
    }

    return err
}

func LoadUnparsedRecipeJobs(ctx context.Context) (error) {
    rows, err := db.Pool.Query(ctx,
        `SELECT id, title, text, user_id 
         FROM recipe_jobs 
         WHERE parsed = FALSE`,
    )
    if err != nil {
        return err
    }
    defer rows.Close()

    count := 0

    for rows.Next() {
        var job models.RecipeJob
        if err := rows.Scan(&job.ID, &job.Name, &job.Text, &job.User_id); err != nil {
            return err
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

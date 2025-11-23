package lib

import (
	"log"
	"context"
	"encoding/json"
	"go-guacamole/db"
	"go-guacamole/models"
)

type ParsedIngredient struct {
    Name             string `json:"name"`
    Amount           string `json:"amount"`
    PreparationNotes string `json:"preparation_notes"`
    IngredientId     int `json:"ingredient_id"`
}

type RecipeResponse struct {
    ID          int                 `json:"id"`
    Title       string              `json:"title"`
    Steps       map[string][]string `json:"steps"`      
    Ingredients []ParsedIngredient  `json:"ingredients"`
}

func SaveParsedRecipe(ctx context.Context, title string, parsed *RecipeParsed) error {
	pool := db.Pool

	steps, err := json.Marshal(parsed.Steps)
        if err != nil {
                return err
        }

	var recipeID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO recipes (title, steps) VALUES ($1, $2) RETURNING id`,
		title, 
		steps,
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
		log.Println("HELLO INGREDIENTS", ing)
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


func GetAllRecipes(ctx context.Context) ([]RecipeResponse, error) {
   rows, err := db.Pool.Query(ctx, `
        SELECT r.id, r.title, r.steps, 
               json_agg(json_build_object(
                   'name', i.name, 
                   'amount', ri.amount, 
                   'preparation_notes', ri.prep_notes,
		   'ingredient_id', i.id
               )) as ingredients
        FROM recipes r
        LEFT JOIN recipe_ingredient ri ON r.id = ri.recipe_id
	LEFT JOIN ingredients i on ri.ingredient_id = i.id
        GROUP BY r.id
    `)
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

func CreateRecipeJob(ctx context.Context, name, text string) (int, error) {
    var jobID int
    err := db.Pool.QueryRow(ctx,
        `INSERT INTO recipe_jobs (title, text, parsed)
         VALUES ($1, $2, FALSE)
         RETURNING id`,
        name, text,
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
        `SELECT id, title, text 
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
        if err := rows.Scan(&job.ID, &job.Name, &job.Text); err != nil {
            return err
        }

        // Queue the job directly
        RecipeQueue <- job
        count++
    }

    log.Printf("Queued %d unparsed recipe jobs\n", count)
    return nil
}



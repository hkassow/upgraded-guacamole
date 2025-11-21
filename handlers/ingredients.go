package handlers

import (
	"encoding/json"
        "net/http"
	"context"
	"log"

    	"go-guacamole/db"
	"go-guacamole/lib"
)

type IngredientUpdate struct {
    ID       int    `json:"id"`
    Category string `json:"category"`
    Location string `json:"location"`
    Season   string `json:"season"`
}

func IngredientsHandler(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        switch r.Method {
        case http.MethodGet:
                handleGetIngredients(w, r)
        case http.MethodPost:
                handlePostIngredients(w, r)
        default:
                http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        }
}

func handleGetIngredients(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    ingredients, err := lib.GetAllIngredients(ctx)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(ingredients)
}

func handlePostIngredients(w http.ResponseWriter, r *http.Request) {
    var updates []IngredientUpdate

    if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    ctx := context.Background()

    for _, ing := range updates {
        // Update season field if provided
        if ing.Season != "" {
            _, err := db.Pool.Exec(ctx,
                `UPDATE ingredients SET season = $1 WHERE id = $2`,
                ing.Season, ing.ID,
            )
            if err != nil {
                log.Println("Season update failed:", err)
                continue
            }
        }

	log.Println("ingredients: ", ing)

        // If category or location is provided → upsert into grocery_tag table
        if ing.Category != "" || ing.Location != "" {
            _, err := db.Pool.Exec(ctx, `
                INSERT INTO grocery_tag (ingredient_id, category, location)
                VALUES ($1, $2, $3)
                ON CONFLICT (ingredient_id)
                DO UPDATE SET
                    category = EXCLUDED.category,
                    location = EXCLUDED.location
            `,
                ing.ID, ing.Category, ing.Location,
            )
            if err != nil {
                log.Println("Tag upsert failed:", err)
                continue
            }
        }
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "message": "Ingredients updated successfully",
    })
}

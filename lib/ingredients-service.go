package lib

import (
        "context"
        "go-guacamole/db"
)

type IngredientWithTag struct {
    ID       int64   `json:"id"`
    Name     string  `json:"name"`
    Season   *string `json:"season,omitempty"`
    Category *string `json:"category,omitempty"`
    Location *string `json:"location,omitempty"`
}

func GetAllIngredients(ctx context.Context) ([]IngredientWithTag, error) {
    query := `
        SELECT
            i.id,
            i.name,
            i.season,
            g.category,
            g.location
        FROM ingredients i
        LEFT JOIN grocery_tag g ON g.ingredient_id = i.id
        ORDER BY i.name
    `

    rows, err := db.Pool.Query(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    ingredients := []IngredientWithTag{}

    for rows.Next() {
        var ing IngredientWithTag

        err := rows.Scan(
            &ing.ID,
            &ing.Name,
            &ing.Season,
            &ing.Category,
            &ing.Location,
        )
        if err != nil {
            return nil, err
        }

        ingredients = append(ingredients, ing)
    }

    return ingredients, nil
}

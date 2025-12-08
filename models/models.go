package models

type RecipeJob struct {
	ID int
	Name string
	Text string
}

type UpdateRecipeRequest struct {
    UpdatedIngredients []UpdatedIngredient `json:"updated_ingredients"`
    UpdatedSteps       []UpdatedStep       `json:"updated_steps"`
    RecipeID 	       int		   `json:"recipe_id"`
}

type UpdatedIngredient struct {
    Name               string `json:"name"`
    Amount             string `json:"amount"`
    PreparationNotes   string `json:"preparation_notes"`
    IngredientID       int    `json:"ingredient_id"`
    RecipeIngredientID int    `json:"recipe_ingredient_id"`
}

type UpdatedStep struct {
    StepName      string `json:"step_name"`
    OriginalSteps string `json:"original_steps"`
    NewSteps      string `json:"new_steps"`
}

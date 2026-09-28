package models

type RecipeJob struct {
    ID int
    Name string
    Text string
    Images   []string
    Type    string
    User_id int
    ParsedJSON []byte // parse result already returned by the model, reused on retry
    Transcript string // image jobs: text already read from the photos, reused on retry
}

type RawIngredient struct {
    Name             	string `json:"name"`
    Amount           	string `json:"amount"`
    PreparationNotes 	string `json:"preparation_notes"`
}

type RawRecipe struct {
    Name        string          `json:"name"`
    Text        string          `json:"text"`
    Images      []string        `json:"images"`
    Ingredients []RawIngredient `json:"ingredients"`
    Type        string          `json:"type"`
}

type UpdateRecipeRequest struct {
    UpdatedIngredients []UpdatedIngredient `json:"updated_ingredients"`
    UpdatedSteps       []UpdatedStep       `json:"updated_steps"`
    Tags               *[]string           `json:"tags,omitempty"` // nil = unchanged, [] = remove all
    RecipeID 	       int		   `json:"recipe_id"`
}

type UpdatedIngredient struct {
    Name               string `json:"name"`
    Amount             string `json:"amount"`
    PreparationNotes   string `json:"preparation_notes"`
    Component          string `json:"component"`
    IngredientID       int    `json:"ingredient_id"`
    RecipeIngredientID int    `json:"recipe_ingredient_id"`
}

type UpdatedStep struct {
    StepName      string `json:"step_name"`
    OriginalSteps string `json:"original_steps"`
    NewSteps      string `json:"new_steps"`
}

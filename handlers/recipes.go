package handlers

import (
    "encoding/json"
    "net/http"

	"go-guacamole/lib"
	"go-guacamole/models"
)

type RawRecipe struct {
    Name     string `json:"name"`
    Text     string `json:"text"`


}

func respondJSON(w http.ResponseWriter, data interface{}) {
    json.NewEncoder(w).Encode(data)
}

func RecipesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		handleGetRecipes(w, r)
	case http.MethodPost:
		handlePostRecipe(w, r)
	case http.MethodPatch:
		handlePatchRecipe(w,r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleGetRecipes(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

	userID := lib.GetUserID(r, store)
    recipes, err := lib.GetAllRecipes(ctx, userID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(recipes)
}

func handlePostRecipe(w http.ResponseWriter, r *http.Request) {
	var rawRecipe RawRecipe
	if err := json.NewDecoder(r.Body).Decode(&rawRecipe); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if rawRecipe.Name == "" || rawRecipe.Text == "" {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}
	
	userID := lib.GetUserID(r, store)
	lib.RecipeQueue <- models.RecipeJob{
        	Name: rawRecipe.Name,
        	Text: rawRecipe.Text,
			User_id: userID,
    }

    w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
        "message": "Recipe queued to be parsed",
    })
}
func handlePatchRecipe(w http.ResponseWriter, r *http.Request) {
	var updateReq models.UpdateRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ctx := r.Context()	
	err := lib.UpdateRecipe(ctx, updateReq.RecipeID, updateReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
    	w.Write([]byte(`{"status":"ok"}`))
}

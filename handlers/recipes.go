package handlers

import (
    "encoding/json"
    "net/http"

	"go-guacamole/lib"
	"go-guacamole/models"
)

type DeleteRecipeRequest struct {
    RecipeID int `json:"recipe_id"`
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
	case http.MethodDelete:
	    handleDeleteRecipe(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleGetRecipes(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

	recipesOf := r.URL.Query().Get("recipes_of")
	
	userID := 0
	if recipesOf != "" {
		userID = lib.GetUserIdByUuid(ctx, recipesOf)
	}

	if recipesOf == "" || userID == 0 {
		userID = lib.GetUserID(r, store)
	}
    recipes, err := lib.GetAllRecipes(ctx, userID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(recipes)
}

func handlePostRecipe(w http.ResponseWriter, r *http.Request) {
	var rawRecipe models.RawRecipe
	if err := json.NewDecoder(r.Body).Decode(&rawRecipe); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if rawRecipe.Name == "" || rawRecipe.Text == "" {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}
	
	userID := lib.GetUserID(r, store)
	if rawRecipe.Type == "manual" {
		ctx := r.Context()	
		err := lib.HandleManualRecipePost(ctx, userID, rawRecipe)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}


		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
    	    "message": "Recipe created",
    	})
	} else {
	
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

func handleDeleteRecipe(w http.ResponseWriter, r *http.Request) {
	var req DeleteRecipeRequest

    err := json.NewDecoder(r.Body).Decode(&req)
    if err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    recipeID := req.RecipeID

	ctx := r.Context()
	userID := lib.GetUserID(r, store)

	err = lib.DeleteRecipe(ctx, recipeID, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"ok"}`))
}

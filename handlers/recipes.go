package handlers

import (
        "encoding/json"
        "net/http"
	"log"

	"go-guacamole/lib"
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
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleGetRecipes(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    recipes, err := lib.GetAllRecipes(ctx)
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
	
	parsed, err := lib.ParseRecipeCall(rawRecipe.Text)
    	if err != nil {
        	log.Println("Error parsing recipe:", err)
        	http.Error(w, "Failed to parse recipe", http.StatusInternalServerError)
        	return
    	}

	if err := lib.SaveParsedRecipe(r.Context(), rawRecipe.Name, parsed); err != nil {
		http.Error(w, "DB save failed: "+err.Error(), 500)
		return
	}

    	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
        	"message": "Recipe added successfully",
    	})
}

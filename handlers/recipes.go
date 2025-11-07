package handlers

import (
        "encoding/json"
        "net/http"
	"log"
)

type Recipe struct {
    Name     string `json:"name"`
}

var recipes = []Recipe{
    {"Spaghetti Carbonara"},
    {"Beef Stroganoff"},
    {"Chicken Curry"},
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
    respondJSON(w, recipes)
}

func handlePostRecipe(w http.ResponseWriter, r *http.Request) {
	log.Println("Hello backend")
	var newRecipe Recipe
	if err := json.NewDecoder(r.Body).Decode(&newRecipe); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if newRecipe.Name == "" {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}

	recipes = append(recipes, newRecipe)

	w.WriteHeader(http.StatusCreated)
	respondJSON(w, map[string]string{
		"message": "Recipe added successfully",
	})
}

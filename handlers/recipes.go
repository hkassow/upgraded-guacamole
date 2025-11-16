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

type Recipe struct {
    Title       string       `json:"title"`
    Steps       []string     `json:"steps"`
}

var recipes = []Recipe{
	{Title: "Spaghetti Carbonara", Steps: []string{"do something", "do nothing"}},
	{Title: "Beef Stroganoff", Steps: []string{"do something", "do nothing"}},
	{Title: "Chicken Curry", Steps: []string{"do something", "do nothing"}},
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
	
	newRecipe := Recipe{
        	Title: rawRecipe.Name,
        	Steps: parsed.Steps,
    	}

	recipes = append(recipes, newRecipe)

    	// Respond
    	w.WriteHeader(http.StatusCreated)
    	respondJSON(w, map[string]string{
        	"message": "Recipe added successfully",
    	})
}

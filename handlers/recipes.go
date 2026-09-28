package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"

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
		handlePatchRecipe(w, r)
	case http.MethodDelete:
		handleDeleteRecipe(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleGetRecipes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var recipes []lib.RecipeResponse
	var err error

	if recipeUUID := r.URL.Query().Get("recipe"); recipeUUID != "" {
		// a shared recipe link: just that one recipe, no login needed
		recipes, err = getSharedRecipe(ctx, recipeUUID)
	} else {
		recipesOf := r.URL.Query().Get("recipes_of")

		userID := 0
		if recipesOf != "" {
			userID = lib.GetUserIdByUuid(ctx, recipesOf)
		}

		if recipesOf == "" || userID == 0 {
			userID = lib.GetUserID(r, store)
		}
		recipes, err = lib.GetAllRecipes(ctx, userID)
	}

	if errors.Is(err, errInvalidRecipeLink) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if errors.Is(err, lib.ErrRecipeNotFound) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// relative to whoever is logged in (not the recipes_of user), so a share link never shows edit controls
	viewerID := lib.GetUserID(r, store)
	for i := range recipes {
		recipes[i].IsMine = viewerID != 0 && recipes[i].OwnerID == viewerID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recipes)
}

var errInvalidRecipeLink = errors.New("invalid recipe link")

// getSharedRecipe loads the recipe from a share link, as a one-item list so the page can render it
// the same way as the normal list.
func getSharedRecipe(ctx context.Context, recipeUUID string) ([]lib.RecipeResponse, error) {
	if _, err := uuid.Parse(recipeUUID); err != nil {
		return nil, errInvalidRecipeLink
	}
	recipe, err := lib.GetRecipeByUUID(ctx, recipeUUID)
	if err != nil {
		return nil, err
	}
	return []lib.RecipeResponse{recipe}, nil
}

func handlePostRecipe(w http.ResponseWriter, r *http.Request) {
	var rawRecipe models.RawRecipe
	if err := json.NewDecoder(r.Body).Decode(&rawRecipe); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if rawRecipe.Name == "" {
		http.Error(w, "Recipe name is required", http.StatusBadRequest)
		return
	}

	// check to ensure that for each submission type the required fields are there
	switch rawRecipe.Type {
	case "text":
		if rawRecipe.Text == "" {
			http.Error(w, "Recipe text is required", http.StatusBadRequest)
			return
		}
	case "image":
		if len(rawRecipe.Images) == 0 {
			http.Error(w, "At least one recipe image is required", http.StatusBadRequest)
			return
		}
	case "manual":
		if rawRecipe.Text == "" {
			http.Error(w, "Recipe instructions are required", http.StatusBadRequest)
			return
		}
		if len(rawRecipe.Ingredients) == 0 {
			http.Error(w, "At least one ingredient is required", http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "Invalid recipe type", http.StatusBadRequest)
		return
	}

	userID := lib.GetUserID(r, store)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	log.Printf("Incoming /recipes request - Name: %s, User: %d, Type: %s, ContentLength: %d\n", rawRecipe.Name, userID, rawRecipe.Type, r.ContentLength)

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
	} else if rawRecipe.Type == "text" {
		err := lib.QueueRecipeJob(r.Context(), models.RecipeJob{
			Name:    rawRecipe.Name,
			Text:    rawRecipe.Text,
			Type:    "text",
			User_id: userID,
		})
		if err != nil {
			http.Error(w, "Failed to queue recipe", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Recipe queued to be parsed",
		})
	} else if rawRecipe.Type == "image" {
		err := lib.QueueRecipeJob(r.Context(), models.RecipeJob{
			Name:    rawRecipe.Name,
			Images:  rawRecipe.Images,
			Type:    "image",
			User_id: userID,
		})
		if err != nil {
			http.Error(w, "Failed to queue recipe", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Recipe image queued to be parsed",
		})

	}
}
func handlePatchRecipe(w http.ResponseWriter, r *http.Request) {
	var updateReq models.UpdateRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	userID := lib.GetUserID(r, store)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	err := lib.UpdateRecipe(ctx, updateReq.RecipeID, userID, updateReq)
	if errors.Is(err, lib.ErrRecipeNotFound) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if errors.Is(err, lib.ErrInvalidTags) || errors.Is(err, lib.ErrInvalidTitle) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
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

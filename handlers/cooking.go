package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func CookingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	title := strings.TrimPrefix(r.URL.Path, "/cooking/")

	title, err := url.PathUnescape(title)
	if err != nil {
		http.Error(w, "invalid recipe title", http.StatusBadRequest)
		return
	}

	log.Printf("Someone is cooking: %s", title)

	response := map[string]string{
		"message": "Cooking " + title,
	}

	json.NewEncoder(w).Encode(response)
}
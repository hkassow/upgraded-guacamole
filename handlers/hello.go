package handlers

import (
	"encoding/json"
	"net/http"
	"log"
	"strings"
	"fmt"
)

type Response struct {
	Message string `json:"message"`
}
func Hello(name string) string {
    message := fmt.Sprintf("Hello hunter, %v. Welcome!", name)
    return message
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := strings.TrimPrefix(r.URL.Path, "/hello/")
	if name == "" {
		name = "World"
	}
	
	response := Response{
		Message: Hello(name),
	}
	
	json.NewEncoder(w).Encode(response)

	log.Println("Hello backend")
}

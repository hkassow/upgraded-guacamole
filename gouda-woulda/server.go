package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

type PredictRequest struct {
	Input string `json:"input"`
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaResponse struct {
	Response string `json:"response"`
}

type PredictResponse struct {
	Result string `json:"result"`
}

func predictHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PredictRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Prepare request for Ollama
	ollamaReq := OllamaRequest{
		Model:  "llama3.2",
		Prompt: req.Input,
	}

	ollamaJSON, _ := json.Marshal(ollamaReq)

	resp, err := http.Post(
		"http://localhost:11434/api/generate",
		"application/json",
		bytes.NewBuffer(ollamaJSON),
	)
	if err != nil {
		http.Error(w, "Failed to contact model: "+err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	// Ollama streams responses — collect all of it
	body, _ := io.ReadAll(resp.Body)

	// Send raw output back to client
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func main() {
	http.HandleFunc("/predict", predictHandler)

	log.Println("Starting HTTPS server on port 443...")
	err := http.ListenAndServeTLS(
		":443",
		"server.crt",
		"server.key",
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
}


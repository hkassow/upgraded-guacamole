package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

type ParseRequest struct {
	Prompt string `json:"prompt"`
}

type ParseImageRequest struct {
	Image string `json:"image"` // base64-encoded image, no data: prefix
}

type OllamaRequest struct {
	Model     string    `json:"model"`
	Think     bool      `json:"think"`
	Stream    bool      `json:"stream"`
	Messages  []Message `json:"messages"`
	KeepAlive string    `json:"keep_alive,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
    Images []sring `json:"images,omitempty"`
}

type OllamaResponse struct {
	Model string `json:"model"`

	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`

	Done bool `json:"done"`
}

func callOllama(model, prompt string, images []string) (string, error) {
	msg := Message{Role: "user", Content: prompt}
	if len(images) > 0 {
		msg.Images = images
	}

	ollamaReq := OllamaRequest{
		Model:     model,
		Think:     false,
		Stream:    false,
		Messages:  []Message{msg},
		KeepAlive: "1m", // unload shortly after use so the two models don't fight for memory
	}

	jsonReq, _ := json.Marshal(ollamaReq)

	resp, err := http.Post(
		"http://localhost:11434/api/chat",
		"application/json",
		bytes.NewBuffer(jsonReq),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", err
	}

	return ollamaResp.Message.Content, nil
}

func parseHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Hello from the ai server")

	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "test-api-key" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Prompt: %s\n", req.Prompt)

	result, err := callOllama("recipe-parser", req.Prompt, nil)
	if err != nil {
		http.Error(w, "Failed to contact praser model: "+err.Error(), 500)
		return
	}

	log.Printf("Output: %s\n", result)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"result": result,
	})
}

func parseImageHandler(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "test-api-key" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ParseImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Step 1: vision model transcribes the image to raw text
	rawText, err := callOllama("recipe-vision", "Transcribe the recipe in this image.", []string{req.Image})
	if err != nil {
		http.Error(w, "Failed to contact vision model: "+err.Error(), 500)
		return
	}
	log.Printf("Transcribed text: %s\n", rawText)

	// Step 2: existing text parser turns raw text into structured JSON
	result, err := callOllama("recipe-parser", rawText, nil)
	if err != nil {
		http.Error(w, "Failed to contact parser model: "+err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"result": result,
	})
}

func main() {
	http.HandleFunc("/parse-recipe", parseHandler)
    http.HandleFunc("/parse-recipe-image", parseImageHandler)


	log.Println("Starting HTTP server on port 8556...")
	err := http.ListenAndServe(":8556", nil)
	if err != nil {
		log.Fatal(err)
	}
}

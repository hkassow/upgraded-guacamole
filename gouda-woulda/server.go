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

type OllamaRequest struct {
	Model string `json:"model"`
	Think bool `json:"think"`
	Stream bool `json:"stream"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaResponse struct {
	Model string `json:"model"`

	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`

	Done bool `json:"done"`
}

func parseHandler(w http.ResponseWriter, r *http.Request) {
    log.Println("Hello from the ai server")
    // ---- API KEY CHECK ----
    apiKey := r.Header.Get("X-API-Key")
    if apiKey != "test-api-key" {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    // ------------------------

    if r.Method != http.MethodPost {
        http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
        return
    }

    var req ParseRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    log.Printf("Prompt: %s\n", string(req.Prompt))

    ollamaReq := OllamaRequest{
	Model:  "recipe-parser",
	Think: false,
	Stream: false,
	Messages: []Message{
	    {
		Role: "user",
		Content: req.Prompt,
	    },
	},
    }

    jsonReq, _ := json.Marshal(ollamaReq)

    resp, err := http.Post(
        "http://localhost:11434/api/chat",
        "application/json",
        bytes.NewBuffer(jsonReq),
    )
    if err != nil {
        http.Error(w, "Failed to contact model: "+err.Error(), 500)
        return
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)


    log.Printf("Final body %s\n", string(body))
    var ollamaResp OllamaResponse

    if err := json.Unmarshal(body, &ollamaResp); err != nil {
    	http.Error(w, "Failed to parse model response", 500)
	return
    }
    
    log.Printf("Output: %s\n", ollamaResp.Message.Content)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
	    "result": ollamaResp.Message.Content,
    })
}

func main() {
	http.HandleFunc("/parse-recipe", parseHandler)

	log.Println("Starting HTTP server on port 8556...")
	err := http.ListenAndServe(":8556", nil)
	if err != nil {
		log.Fatal(err)
	}
}

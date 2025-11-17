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
	Prompt string `json:"prompt"`
	Stream bool `json:"stream"`
}

type OllamaResponse struct {
	Response string `json:"response"`
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
	Model:  "llama3:70b-instruct-q3_K_M",
        Prompt: req.Prompt,
	Stream: false,
    }

    jsonReq, _ := json.Marshal(ollamaReq)

    resp, err := http.Post(
        "http://localhost:11434/api/generate?stream=false",
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
    
    log.Printf("Output: %s\n", ollamaResp)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
	    "result": ollamaResp.Response,
    })
}

func main() {
	http.HandleFunc("/parse-recipe", parseHandler)

	log.Println("Starting HTTPS server on port 8556...")
	err := http.ListenAndServeTLS(
		":8556",
		"server.crt",
		"server.key",
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
}

func predictHandler(w http.ResponseWriter, r *http.Request) {
    // ---- API KEY CHECK ----
    apiKey := r.Header.Get("X-API-Key")
    if apiKey != InternalAPIKey {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    // ------------------------

    if r.Method != http.MethodPost {
        http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
        return
    }

    var req PredictRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    ollamaReq := OllamaRequest{
        Model:  "llama3.2",
        Prompt: req.Input,
    }

    jsonReq, _ := json.Marshal(ollamaReq)

    resp, err := http.Post(
        "http://localhost:11434/api/generate",
        "application/json",
        bytes.NewBuffer(jsonReq),
    )
    if err != nil {
        http.Error(w, "Failed to contact model: "+err.Error(), 500)
        return
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

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

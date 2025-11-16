package lib

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
	"log"
)

type Ingredient struct {
	Name   string `json:"name"`
	Amount string `json:"amount"`
}

type RecipeParsed struct {
	Steps       []string     `json:"steps"`
	Ingredients []Ingredient `json:"ingredients"`
}

type ParseRequest struct {
	Prompt string `json:"text"`
}
func ParseRecipeCall(recipeText string) (*RecipeParsed, error) {
	apiKey, err := LoadSecret("INTERNAL_API_KEY")
        if err != nil {
            log.Fatal(err)
        }
	gouda_ip, err := LoadSecret("GOUDA_IP")
	if err != nil {
	    log.Fatal(err)
	}
	log.Println("secrets", apiKey, gouda_ip)

	prompt := fmt.Sprintf(`
	    Return ONLY valid JSON.
	    
	    Parse the following recipe and output:
	    {
	      "steps": [...],
	      "ingredients": [
	         { "name": "string", "amount": "string" }
	      ]
	    }
	    
	    Rules:
	    - Normalize ingredient names (remove brand adjectives)
	    - Preserve preparation (e.g. "thinly sliced", "melted")
	    - Steps must be short instructions
	    
	    Recipe:
	    %s
	`, recipeText)

	body, _ := json.Marshal(ParseRequest{
		Prompt: prompt,
	})

	path := fmt.Sprintf("https://%v:443/parse-recipe", gouda_ip)
	
	// setup http client

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	
	parsed := &RecipeParsed{}

	req, err := http.NewRequest("POST", path, bytes.NewBuffer(body))
	if err != nil {
		return parsed, err
	}
	
	return parsed, fmt.Errorf("test error to stop process")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey) // internal auth header

	resp, err := client.Do(req)

	if err != nil {
		return parsed, fmt.Errorf("ollama request failed: %w", err)
	}

	defer resp.Body.Close()

	data, _ := ioutil.ReadAll(resp.Body)

	log.Println("HELLO OLLAMA", data)

	if err := json.Unmarshal(data, &parsed); err != nil {
		return parsed, err
	}

	return parsed, nil
}


package lib

import (
	"bytes"
	"crypto/tls"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"net/http"
	"time"
	"log"
	"go-guacamole/models"
)

type Ingredient struct {
    Name   string `json:"name"`
    Amount string `json:"amount"`
    PreparationNotes string `json:"preparation_notes"`
}

type RecipeParsed struct {
    Steps       map[string][]string `json:"steps"`
    Ingredients []Ingredient `json:"ingredients"`
}

type Request struct {
    Prompt string `json:"prompt"`
}

type ModelResponse struct {
    Result string `json:"result"`
}

func cleanupJSON(raw string) string {
    raw = strings.TrimSpace(raw)

    raw = strings.TrimPrefix(raw, "```json")
    raw = strings.TrimPrefix(raw, "```")
    raw = strings.TrimSuffix(raw, "```")

    return strings.TrimSpace(raw)
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

	body, _ := json.Marshal(Request{
		Prompt: recipeText,
	})
	
	path := fmt.Sprintf("https://%v:8556/parse-recipe", gouda_ip)
	
	// setup http client

	client := &http.Client{
		Timeout: 240000 * time.Second,
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
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey) // internal auth header
	
	resp, err := client.Do(req)

	if err != nil {
	    return parsed, fmt.Errorf("ollama request failed: %w", err)
	}

	defer resp.Body.Close()



	data, _ := io.ReadAll(resp.Body)

	
    	var wrapper ModelResponse
    	if err := json.Unmarshal(data, &wrapper); err != nil {
    	    return parsed, fmt.Errorf("failed to decode wrapper: %w", err)
    	}

    	clean := cleanupJSON(wrapper.Result)

    	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
    	    return parsed, fmt.Errorf("failed to parse recipe json: %w", err)
    	}
	return parsed, nil
}

var RecipeQueue = make(chan models.RecipeJob, 100)

func StartRecipeWorker() {
    go func() {
        for job := range RecipeQueue {
	    log.Println("Processing recipe:", job.Name)

	    ctx := context.Background()

            jobID := job.ID
            if jobID == 0 {
                var err error
                jobID, err = CreateRecipeJob(ctx, job.Name, job.Text)
                if err != nil {
                    continue
                }
            }

            parsed, err := ParseRecipeCall(job.Text)
            if err != nil {
                log.Println("Error parsing recipe:", err)
                continue
            }

            if err := SaveParsedRecipe(context.Background(), job.Name, parsed); err != nil {
                log.Println("Error saving recipe:", err)
                continue
            }

            _ = MarkRecipeJobParsed(ctx, jobID)

            log.Println("Recipe saved successfully:", job.Name)
        }
    }()
}

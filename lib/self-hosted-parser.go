package lib

// Client for the self-hosted model server in gouda-woulda/. Not currently used - the worker
// calls DeepInfra instead (see ProcessRecipeJob) - but kept so it can be switched back.

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type ImageRequest struct {
	Images []string `json:"images"`
}

type Request struct {
	Prompt string `json:"prompt"`
}

type ModelResponse struct {
	Result string `json:"result"`
}

func callParserEndpoint(path string, body []byte) (*RecipeParsed, error) {
	apiKey, err := LoadSecret("INTERNAL_API_KEY")
	if err != nil {
		log.Fatal(err)
	}

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
	req.Header.Set("X-API-Key", apiKey)

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

func ParseRecipeCall(recipeText string) (*RecipeParsed, error) {
	gouda_ip, err := LoadSecret("GOUDA_IP")
	if err != nil {
		log.Fatal(err)
	}

	body, _ := json.Marshal(Request{
		Prompt: recipeText,
	})

	path := fmt.Sprintf("http://%v:8556/parse-recipe", gouda_ip)

	return callParserEndpoint(path, body)
}

func ParseRecipeImageCall(images []string) (*RecipeParsed, error) {
	gouda_ip, err := LoadSecret("GOUDA_IP")
	if err != nil {
		log.Fatal(err)
	}

	body, _ := json.Marshal(ImageRequest{
		Images: images,
	})

	path := fmt.Sprintf("http://%v:8556/parse-recipe-image", gouda_ip)

	return callParserEndpoint(path, body)
}

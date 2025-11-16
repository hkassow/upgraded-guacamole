package lib

import (
	"context"
	"fmt"
	"log"
)

// ParseRecipe sends raw recipe text to ChatGPT and returns the response text.
func ParseRecipe(recipeText string) (string, error) {
	apiKey, err := LoadSecret("INTERNAL_API_KEY")
	if err != nil {
	    log.Fatal(err)
	}

	// Prompt: Ask model to parse the recipe and output structured JSON
	prompt := fmt.Sprintf(`
	    Parse the following recipe text and return a JSON with only these fields:
	    1. steps: an array of strings describing each step
	    2. ingredients: an array of objects with fields:
   		- name: normalized ingredient name (remove adjectives like "fresh", "organic" etc., unify to canonical names)
   		- amount: the quantity (e.g., "2 cups", "150g") 
		- preparation notes like "thinly sliced" or "melted" if available

	    Recipe text:
	    %s

	    Output strictly valid JSON, no extra text.
	`, recipeText)

	
	log.Println("Hello recipe parser", recipeText)

	if err != nil {
	    return "", err
	}

	if len(resp.Choices) == 0 {
	    return "", fmt.Errorf("no response from ChatGPT")
	}

	return resp.Choices[0].Message.Content, nil
}


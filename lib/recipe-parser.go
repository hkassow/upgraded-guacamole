package lib

import (
	"bytes"
	"crypto/tls"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"net/http"
	"time"
	"log"
	"go-guacamole/models"
)

const (
	deepInfraChatURL = "https://api.deepinfra.com/v1/openai/chat/completions"
 
	deepInfraTextModel = "Qwen/Qwen3-235B-A22B-Instruct-2507" // .09 in .55 out
	//deepInfraTextModel = "Qwen/Qwen3-32B"				 // .08 in .28 out
 
	deepInfraVisionModel = "Qwen/Qwen3-VL-235B-A22B-Instruct" // .20 in .88 out
	//deepInfraVisionModel = "Qwen3-VL-30B-A3B-Instruct" // .15 in .60 out
)
const recipeTextSystemPrompt = `
Your task is to extract structured recipe data from raw text.
 
**INPUT:** You will receive raw recipe text that may contain:
- A list of ingredients
- Cooking steps/instructions
- Optional component sections (sauce, marinade, etc.)
 
**OUTPUT:** You must extract ALL ingredients and ALL steps from the input text.
 
Return **only valid JSON**, using the following schema:
 
{
  "steps": {
    "main": ["string", "string", ...],
    "component_name": ["string", ...]   // optional
  },
  "ingredients": [
    {
      "name": "string",
      "amount": "string",
      "alt_amount": "string",
      "preparation_notes": "string"
    }
  ]
}
 
### EXTRACTION REQUIREMENTS
1. Extract EVERY step from the input text - do not skip or omit any steps.
2. Extract EVERY ingredient listed in the input's ingredients list section - do not skip or omit any ingredients. Do NOT create additional ingredient entries from mentions inside the steps (see INGREDIENT RULE 7 below).
3. If no steps are found, return: "steps": {"main": []}
4. If no ingredients are found, return: "ingredients": []
5. Do not add, invent, or infer steps or ingredients that are not in the original text.
 
### STEP PARSING RULES
 
1. Preserve each step EXACTLY as written.
   - Do NOT shorten, simplify, summarize, or rewrite steps.
   - Only convert them into JSON strings in their original form.
 
2. Group steps by components when they exist.
   Examples of components:
   - "sauce"
   - "marinade"
   - "topping"
   - "dough"
   - "batter"
   - "filling"
   - "frosting"
   - "glaze"
 
3. The main cooking process belongs under:
   "main": [ ... ]
 
4. Component detection rules:
   - If the recipe contains headings like "For the sauce", "Make the marinade", etc.,
     create a JSON key using the component name, e.g.:
       "sauce": [ ... steps ... ]
   - If no component headings are present, **all steps go under ` + "`\"main\"`" + `**.
 
5. Remove numbering (e.g. "1.", "Step 1", "•") but keep the original sentences.
 
6. Each step must remain a complete, standalone instruction.
 
---
 
### INGREDIENT RULES
0. Remove non-essential information from all fields:
   - Remove parenthetical notes about substitutions, omissions, or recipe notes
     * "(Note 5 to omit)" → remove entirely
     * "(see notes)" → remove entirely
     * "(optional)" → add "optional" to preparation_notes if meaningful
   - Move alternative/equivalent measurements to ` + "`\"alt_amount\"`" + ` instead of discarding them:
     * The FIRST amount written (outside parentheses, before any "/") is the primary
       amount and goes in ` + "`\"amount\"`" + `.
     * A SECOND amount for the same ingredient - in parentheses, or after a "/" -
       is an alternative/equivalent measurement and goes in ` + "`\"alt_amount\"`" + `,
       written exactly as it appears (unit included).
     * "200g / 7 oz" → amount: "200g", alt_amount: "7 oz"
     * "1 cup / 240ml" → amount: "1 cup", alt_amount: "240ml"
     * "2¾ (650ml) cups" → amount: "2¾ cups", alt_amount: "650ml"
     * If there is no second measurement, leave ` + "`\"alt_amount\": \"\"`" + `.
   - Remove recipe cross-references:
     * "(Note 5)", "(see step 3)", "(*))" → remove entirely
 
1. Normalize ingredient names:
   - Remove brand or marketing words (e.g., "Organic Kirkland Butter" → "butter").
   - Remove cut/style descriptors that describe the type or cut of the ingredient:
     * "streaky bacon" → name: "bacon", add "streaky" to preparation_notes
     * "ribeye steak" → name: "steak", add "ribeye cut" to preparation_notes
     * "russet potatoes" → name: "potatoes", add "russet" to preparation_notes
     * "roma tomatoes" → name: "tomatoes", add "roma" to preparation_notes
     * "yellow onion" → name: "onion", add "yellow" to preparation_notes
   - Keep only the base ingredient name in the ` + "`\"name\"`" + ` field.
   - Move removed descriptors to ` + "`\"preparation_notes\"`" + `.
 
2. Preserve preparation descriptors:
   - "thinly sliced", "softened", "beaten", "melted", "chopped", "diced", etc.
   - Remove from ingredient name, but preserve in preparation_notes
   - If multiple preparation notes exist, separate them with commas:
     * "streaky bacon, chopped" → preparation_notes: "streaky, chopped"
     * "yellow onion, diced finely" → preparation_notes: "yellow, diced finely"
 
3. Extract quantities:
   - See rule 0 above for how to split a primary amount from an alt_amount when
     two measurements are given. This rule covers the primary amount only.
   - Extract ONLY the numeric amount and unit of measurement
     * Valid: "200g", "2 cups", "1/2 tsp", "½ cup", "3 tbsp"
     * Invalid: "½ cup finely shredded" (remove "finely shredded")
   - Stop extracting at the first preparation word:
     * Preparation words: "finely", "coarsely", "thinly", "roughly", "chopped",
       "diced", "sliced", "shredded", "grated", "minced", "crushed", "beaten",
       "melted", "softened", "cubed", "peeled", etc.
   - Everything after the unit goes into preparation_notes, NOT amount
   - If quantity exists: put it into ` + "`\"amount\"`" + `.
   - If none exists, leave ` + "`\"amount\": \"\"`" + `.
 
4. Extract preparation notes:
   - Combine all preparation descriptors and variety/cut descriptors with commas
   - Order: variety/cut first, then preparation methods
     * Example: "yellow, diced" or "streaky, chopped"
   - If none exist, leave ` + "`\"preparation_notes\": \"\"`" + `.
   - Do NOT include recipe notes, cross-references, or substitution suggestions
 
5. Split combined ingredients:
   - "2 eggs, beaten" → name: "eggs", amount: "2", preparation_notes: "beaten"
 
6. Handle duplicate ingredients:
   - This applies only when the ingredients list section itself lists the same ingredient
     on more than one separate line (e.g. two "butter" lines - one for a sauce, one for
     a dough). It does NOT apply to an ingredient being mentioned again later in the steps
     - see rule 7 for that case.
   - If the ingredients list has the same ingredient on separate lines, create separate entries.
   - Add context to preparation_notes if helpful:
     * First mention: {"name": "butter", "amount": "2 tbsp", "preparation_notes": "for sauce"}
     * Second mention: {"name": "butter", "amount": "1/4 cup", "preparation_notes": "for dough"}
 
7. Do NOT create ingredient entries from the steps:
   - Ingredients come ONLY from the ingredients list section of the input. If the input has
     no distinguishable ingredients list at all (ingredients are only ever described within
     the steps), extract them from the steps instead - this exception is rare.
   - The steps frequently refer back to an ingredient that's already in the ingredients list,
     to describe how much of it to use at that point (e.g. "the remaining sugar", "3
     tablespoons of the sugar", "the vanilla", "half the butter"). These are usage
     instructions, NOT new ingredients - do not create an additional ingredient entry for
     them, even if the wording or amount doesn't exactly match the ingredients list line.
   - Example: ingredients list has "1½ cups sugar". Steps say "3 tablespoons of the sugar"
     (step 1) and "remaining 1¼ cups plus 2 tablespoons sugar" (step 2). Output ONE
     ingredient entry for sugar (from the ingredients list), not three.
 
---
 
### OUTPUT RULES
 
- **Return JSON only**, no explanations or markdown.
- JSON must be valid and parseable.
- Do not invent ingredients or steps.
- Do not create ingredient entries from mentions inside the steps that refer back to an
  ingredient already in the ingredients list (see INGREDIENT RULE 7).
- Do not omit subcomponent steps (e.g., sauces, toppings, fillings).
- Follow this schema strictly.
 
---
 
### AMOUNT EXTRACTION EXAMPLES
 
Input: "½ cup finely shredded cheddar cheese"
- amount: "½ cup"
- alt_amount: ""
- name: "cheddar cheese"
- preparation_notes: "finely shredded"
 
Input: "2 large yellow onions, finely diced"
- amount: "2"
- alt_amount: ""
- name: "onions"
- preparation_notes: "large, yellow, finely diced"
 
Input: "200g / 7 oz streaky bacon, chopped"
- amount: "200g"
- alt_amount: "7 oz"
- name: "bacon"
- preparation_notes: "streaky, chopped"
 
Input: "2¾ (650ml) cups milk whole"
- amount: "2¾ cups"
- alt_amount: "650ml"
- name: "milk"
- preparation_notes: "whole"
`
 
const recipeImageSystemPrompt = `
You will be shown an image of a recipe (photo, screenshot, or recipe card).
Transcribe ALL visible text related to the recipe exactly as written:
title, ingredients, and instructions/steps.
 
Pay special attention to fraction characters (¼, ½, ¾, ⅓, ⅔, ⅛, etc.) and
mixed numbers (e.g. "1¾", "2½"). Transcribe these exactly as they appear —
do not round, guess, or confuse them with whole numbers or other fractions.
If a fraction is unclear or ambiguous, transcribe it as best as possible
rather than omitting it.
 
Do not summarize, reformat, or omit anything.
Do not add commentary or explanations.
Just output the raw transcribed text, preserving structure (e.g. section headings like "For the sauce").
`
 

// ---------------------------------------------------------------------------
// OpenAI-compatible request/response shapes (DeepInfra speaks this dialect)
// ---------------------------------------------------------------------------
 
type diContentPart struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	ImageURL *diImageURL  `json:"image_url,omitempty"`
}
 
type diImageURL struct {
	URL string `json:"url"`
}
 
type diMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string OR []diContentPart
}
 
type diChatRequest struct {
	Model       string      `json:"model"`
	Messages    []diMessage `json:"messages"`
	Temperature float64     `json:"temperature"`
}
 
type diChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type Ingredient struct {
    Name   string `json:"name"`
    Amount string `json:"amount"`
    AltAmount string `json:"alt_amount"`
    PreparationNotes string `json:"preparation_notes"`
}

type RecipeParsed struct {
    Steps       map[string][]string `json:"steps"`
    Ingredients []Ingredient `json:"ingredients"`
}

type ImageRequest struct {
    Images []string `json:"images"`
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

func deepInfraHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 120 * time.Second,
	}
}

func callDeepInfra(req diChatRequest) (string, error) {
	apiKey, err := LoadSecret("DEEPINFRA_API_KEY")
	if err != nil {
		return "", fmt.Errorf("loading DeepInfra API key: %w", err)
	}
 
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling deepinfra request: %w", err)
	}
 
	httpReq, err := http.NewRequest("POST", deepInfraChatURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
 
	resp, err := deepInfraHTTPClient().Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("deepinfra request failed: %w", err)
	}
	defer resp.Body.Close()
 
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading deepinfra response: %w", err)
	}
 
	var wrapper diChatResponse
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return "", fmt.Errorf("failed to decode deepinfra response: %w (raw: %s)", err, string(data))
	}
 
	if wrapper.Error != nil {
		return "", fmt.Errorf("deepinfra error: %s", wrapper.Error.Message)
	}
	if len(wrapper.Choices) == 0 {
		return "", fmt.Errorf("deepinfra returned no choices (raw: %s)", string(data))
	}
 
	return wrapper.Choices[0].Message.Content, nil
}

func extractRecipeJSON(rawText string) (*RecipeParsed, error) {
	parsed := &RecipeParsed{}
 
	content, err := callDeepInfra(diChatRequest{
		Model:       deepInfraTextModel,
		Temperature: 0,
		Messages: []diMessage{
			{Role: "system", Content: recipeTextSystemPrompt},
			{Role: "user", Content: rawText},
		},
	})
	if err != nil {
		return parsed, err
	}
 
	clean := cleanupJSON(content)
	logSchemaDrift(clean)
	if err := json.Unmarshal([]byte(clean), parsed); err != nil {
		return parsed, fmt.Errorf("failed to parse recipe json: %w (raw model output: %s)", err, content)
	}
 
	return parsed, nil
}

func logSchemaDrift(raw string) {
	var generic map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &generic); err != nil {
		return // malformed JSON entirely - the real Unmarshal call will report this
	}
 
	for key := range generic {
		if key != "steps" && key != "ingredients" {
			log.Printf("deepinfra: unexpected top-level field %q in recipe JSON", key)
		}
	}
 
	if rawIngredients, ok := generic["ingredients"]; ok {
		var ingredients []map[string]json.RawMessage
		if err := json.Unmarshal(rawIngredients, &ingredients); err == nil {
			for i, ing := range ingredients {
				for key := range ing {
					if key != "name" && key != "amount" && key != "preparation_notes"  && key != "alt_amount" {
						log.Printf("deepinfra: unexpected field %q in ingredients[%d]", key, i)
					}
				}
			}
		}
	}
}


func ParseRecipeCallDeepInfra(recipeText string) (*RecipeParsed, error) {
	return extractRecipeJSON(recipeText)
}

func imageDataURI(imageBase64 string) string {
	mime := "image/jpeg"
 
	if raw, err := base64.StdEncoding.DecodeString(imageBase64); err == nil {
		detected := http.DetectContentType(raw)
		if len(detected) >= len("image/") && detected[:6] == "image/" {
			mime = detected
		}
	}
 
	return fmt.Sprintf("data:%s;base64,%s", mime, imageBase64)
}

func ParseRecipeImageCallDeepInfra(images []string) (*RecipeParsed, error) {
	if len(images) == 0 {
		return &RecipeParsed{}, fmt.Errorf("no images provided")
	}

	content := make([]diContentPart, 0, len(images)+1)
	for _, img := range images {
		content = append(content, diContentPart{
			Type:     "image_url",
			ImageURL: &diImageURL{URL: imageDataURI(img)},
		})
	}
	content = append(content, diContentPart{
		Type: "text",
		Text: "Transcribe this recipe. If multiple images are provided, treat them as pages of the same recipe.",
	})

	transcript, err := callDeepInfra(diChatRequest{
 		Model:       deepInfraVisionModel,
 		Temperature: 0,
 		Messages: []diMessage{
 			{Role: "system", Content: recipeImageSystemPrompt},
			{
				Role: "user",
				Content: content,
 			},
 		},
 	})

	if err != nil {
		return &RecipeParsed{}, fmt.Errorf("image transcription failed: %w", err)
	}
 
	return extractRecipeJSON(transcript)
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
                jobID, err = CreateRecipeJob(ctx, job)
                if err != nil {
                    continue
                }
            }

            var parsed *RecipeParsed
            var err error

            switch job.Type {
            case "image":
                //parsed, err = ParseRecipeImageCall(job.Images)
				parsed, err = ParseRecipeImageCallDeepInfra(job.Images)
            default: // "text"
                //parsed, err = ParseRecipeCall(job.Text)
				parsed, err = ParseRecipeCallDeepInfra(job.Text)
            }

            if err != nil {
                log.Println("Error parsing recipe:", err)
                continue
            }

            if err := SaveParsedRecipe(context.Background(), job.Name, job.User_id, parsed); err != nil {
                log.Println("Error saving recipe:", err)
                continue
            }

            _ = MarkRecipeJobParsed(ctx, jobID)

            log.Println("Recipe saved successfully:", job.Name)
        }
    }()
}

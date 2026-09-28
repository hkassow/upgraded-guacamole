package lib

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"net/http"
	"time"
	"log"
	"go-guacamole/models"
)

// var (not const) so tests can point it at a fake server
var deepInfraChatURL = "https://api.deepinfra.com/v1/openai/chat/completions"

const (
	deepInfraTextModel = "Qwen/Qwen3-235B-A22B-Instruct-2507" // .09 in .55 out
	//deepInfraTextModel = "Qwen/Qwen3-32B"				 // .08 in .28 out
 
	deepInfraVisionModel = "Qwen/Qwen3-VL-235B-A22B-Instruct" // .20 in .88 out
	//deepInfraVisionModel = "Qwen/Qwen3-VL-30B-A3B-Instruct" // .15 in .60 out
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
      "preparation_notes": "string",
      "component": "string"
    }
  ],
  "ingredients_used_for_step": {
    "main": {
      "1": [{"name": "string", "amount": "string"}],
      "3": [{"name": "string", "amount": "string"}]
    }
	"component_name": {   // optional, same component keys as "steps"
	  "2": [{"name": "string", "amount": "string"}]
    }
  }
}
 
### EXTRACTION REQUIREMENTS
1. Extract EVERY step from the input text - do not skip or omit any steps.
2. Extract EVERY ingredient listed in the input's ingredients list section - do not skip or omit any ingredients. Do NOT create additional ingredient entries from mentions inside the steps (see INGREDIENT RULE 7 below).
3. If no steps are found, return: "steps": {"main": []}
4. If no ingredients are found, return: "ingredients": []
5. Do not add, invent, or infer steps or ingredients that are not in the original text. The ONLY
   exception is the short "Make the <component>" steps described in STEP PARSING RULE 7.
6. If no step uses any ingredients, return: "ingredients_used_for_step": {}
 
### STEP PARSING RULES
 
1. Preserve each step EXACTLY as written.
   - Do NOT shorten, simplify, summarize, or rewrite steps.
   - Only convert them into JSON strings in their original form.
   - (The only added text is the "Make the <component>" steps from rule 7.)
 
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
   - Name the key after the recipe's own heading, lowercase, with words separated by spaces:
     "Cream filling" → "cream filling", "For the whiskey syrup" → "whiskey syrup".

5. Remove numbering (e.g. "1.", "Step 1", "•") but keep the original sentences.

6. Each step must remain a complete, standalone instruction.

7. Point the main steps at the other components:
   - Recipes often list a component (a syrup, filling, sauce...) in its own section - frequently
     at the END of the recipe - even though a "main" step needs it earlier. Without a pointer the
     reader never learns when to make it.
   - For each component other than "main": if a "main" step uses the finished component (e.g.
     "brush the layer with the whiskey syrup", "spoon in the cream filling") and no earlier
     "main" step already says to make it, INSERT one short step into "main" directly BEFORE the
     first "main" step that uses it, worded exactly like:
       "Make the whiskey syrup (see the Whiskey syrup steps)."
     using the component's key, with the first letter capitalized inside the parentheses.
   - Insert at most ONE such step per component. If several components are first used in the
     same step, insert one step for each, in the order they are mentioned.
   - Do NOT move, copy, or change the component's own steps - they stay under its own key.
   - The inserted steps count like any other step when numbering "ingredients_used_for_step",
     and they list no ingredients.
   - Example - the recipe's instructions are:
       1. Bake the cookie layers.
       2. Brush each layer with the whiskey syrup and spread with the cream filling.
       Cream filling: Beat the cream cheese and sugar...
       Whiskey syrup: Simmer the honey and whiskey...
     → "main": [
         "Bake the cookie layers.",
         "Make the whiskey syrup (see the Whiskey syrup steps).",
         "Make the cream filling (see the Cream filling steps).",
         "Brush each layer with the whiskey syrup and spread with the cream filling."
       ],
       "cream filling": ["Beat the cream cheese and sugar..."],
       "whiskey syrup": ["Simmer the honey and whiskey..."]
 
---


### STEP INGREDIENT USAGE RULES
 
"ingredients_used_for_step" records which ingredients are ADDED OR USED in each step,
and how much of each. It is keyed first by the same component keys as "steps" ("main",
"sauce", etc.), then by the step's 1-based position within that component's array
(the first step is "1", the second is "2", and so on - count carefully).
 
1. Only include a step's key if that step actually adds/uses ingredients. Steps with no
   ingredients (e.g. "Preheat the oven", "Let cool for 10 minutes", and the inserted "Make the
   <component>" steps) get NO key at all - do not include empty arrays. Step numbers count
   the inserted "Make the <component>" steps too.
 
2. List an ingredient only in the step where it is added or used from its raw form.
   Once ingredients have been combined into a mixture, later steps that refer to that
   mixture ("the flour mixture", "the batter", "the dough", "the sauce") list NOTHING for
   those ingredients - do not list the individual ingredients again.
 
3. "name" must exactly match the "name" of the corresponding entry in "ingredients".
 
4. "amount" is the amount used IN THAT STEP:
   - If the step states an amount ("3 tablespoons of the sugar", "remaining 1¼ cups plus 2
     tablespoons (270 g) sugar"), use that stated amount, following the same rules as
     ingredient amounts (one primary amount only - drop parenthetical alternatives).
   - If the step uses the ingredient without stating an amount ("add the flour"), use the
     full amount from the ingredients list.
   - If the step says "the rest" or "the remaining" WITHOUT stating an amount, set
     "amount" to "remaining". Do NOT calculate amounts.
 
5. If an ingredient is used in more than one step (e.g. butter in step 1 and step 5),
   list it in each of those steps with that step's amount.
 
6. Do NOT invent ingredients here. Every "name" must correspond to an ingredient that
   exists in the "ingredients" list.
 
Example - given ingredients "1½ cups sugar", "2 cups flour", "1 tsp vanilla extract",
"3 eggs" and these steps:
  1. Put 3 tablespoons of the sugar in a saucepan and cook until dissolved.
  2. Whisk the eggs, the remaining sugar, and the vanilla until pale.
  3. Fold in the flour, then bake for 30 minutes.
  4. Let the cake cool completely.
 
"ingredients_used_for_step": {
  "main": {
    "1": [{"name": "sugar", "amount": "3 tablespoons"}],
    "2": [{"name": "eggs", "amount": "3"}, {"name": "sugar", "amount": "remaining"}, {"name": "vanilla extract", "amount": "1 tsp"}],
    "3": [{"name": "flour", "amount": "2 cups"}]
  }
}
(Step 4 uses no ingredients, so it has no key.)
 
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
   - EXCEPTION - keep the descriptor in the name when it makes a DIFFERENT product that is
     bought separately, not just a variety of the same thing:
     * "granulated sugar", "brown sugar", "dark brown muscovado sugar", "powdered sugar"
     * "heavy cream", "sour cream", "cream cheese"
     * "all-purpose flour", "bread flour", "baking soda", "baking powder"
   - If the ingredients list has the same base ingredient more than once with different
     descriptors, EVERY one of those entries keeps its descriptor in the name so they can be
     told apart - never output two entries that are both just named "sugar":
     * "½ cup granulated sugar" and "⅔ cup dark brown muscovado sugar"
       → names "granulated sugar" and "dark brown muscovado sugar"
 
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
   - Words about HOW to measure ("packed", "heaping", "level", "scant", "generous") are not part
     of the amount - put them in preparation_notes:
     * "packed ⅔ cup dark brown sugar" → amount: "⅔ cup", preparation_notes: "packed"
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
   - Use "component" (rule 8) to say which section each entry is for - do NOT put
     "for sauce" / "for dough" in preparation_notes:
     * First mention: {"name": "butter", "amount": "2 tbsp", "preparation_notes": "", "component": "sauce"}
     * Second mention: {"name": "butter", "amount": "1/4 cup", "preparation_notes": "", "component": "dough"}
 
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

8. Assign each ingredient a "component":
   - "component" MUST be exactly one of the keys you used in "steps" (e.g. "main", "sauce"),
     spelled the same way, lowercase.
   - Use the ingredients list's OWN section headings to decide. If the ingredients list has a
     heading like "For the sauce:" or "Filling", every ingredient under that heading gets
     that component (the same key used for that section in "steps").
   - Ingredients under no heading, or under a heading for the main dish, get "main".
   - If the ingredients list has NO section headings at all, every ingredient gets "main" -
     do NOT guess sections from the steps.
   - If an ingredient is listed under two headings, keep two entries (see rule 6), each
     with its own component.
   - Example - ingredients list:
       500g chicken thighs
       1 onion, diced
       For the sauce:
       2 tbsp soy sauce
       1 tbsp honey
     → chicken thighs and onion get "component": "main"; soy sauce and honey get
       "component": "sauce" (and the sauce steps go under "steps"."sauce").
 
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
	Model         string            `json:"model"`
	Messages      []diMessage       `json:"messages"`
	Temperature   float64           `json:"temperature"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Stream        bool              `json:"stream"`
	StreamOptions *diStreamOptions  `json:"stream_options,omitempty"`
}

type diStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type diUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// one "data:" event of a streamed chat completion
type diStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"` // "length" means the reply was cut off
	} `json:"choices"`
	Usage *diUsage `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type Ingredient struct {
    Name   string `json:"name"`
    Amount string `json:"amount"`
    AltAmount string `json:"alt_amount"`
    PreparationNotes string `json:"preparation_notes"`
    Component string `json:"component"`
}

type StepIngredient struct {
	Name   string `json:"name"`
	Amount string `json:"amount"`
}

type RecipeParsed struct {
    Steps       map[string][]string `json:"steps"`
    Ingredients []Ingredient `json:"ingredients"`
	IngredientsUsedForStep map[string]map[string][]StepIngredient `json:"ingredients_used_for_step"`
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

// DeepInfra replies are streamed, so a slow-but-working generation can be told apart from a stuck
// one: a call fails when no data arrives for a while, not after a fixed total time.
var (
	// waiting for the first data covers prompt processing, which is slow for big photo prompts
	deepInfraFirstDataTimeout = 3 * time.Minute
	// once tokens are flowing, this long without any means the stream is stuck
	deepInfraIdleTimeout = 60 * time.Second
	// hard backstop for a reply that keeps streaming forever
	deepInfraMaxDuration = 15 * time.Minute
	// how often to log progress on a long reply
	deepInfraProgressInterval = 60 * time.Second
)

// Caps a runaway reply (e.g. the model repeating itself). A large recipe's JSON is a few thousand
// tokens; hitting this shows up as finish_reason "length".
const deepInfraMaxTokens = 8192

var deepInfraClient = &http.Client{} // timeouts come from the request context, see callDeepInfra

func callDeepInfra(req diChatRequest) (string, error) {
	apiKey, err := LoadSecret("DEEPINFRA_API_KEY")
	if err != nil {
		return "", fmt.Errorf("loading DeepInfra API key: %w", err)
	}

	req.Stream = true
	req.StreamOptions = &diStreamOptions{IncludeUsage: true}
	if req.MaxTokens == 0 {
		req.MaxTokens = deepInfraMaxTokens
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling deepinfra request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), deepInfraMaxDuration)
	defer cancel()

	// cancels the request if the stream goes quiet; reset every time data arrives
	var stalled atomic.Bool
	idle := time.AfterFunc(deepInfraFirstDataTimeout, func() {
		stalled.Store(true)
		cancel()
	})
	defer idle.Stop()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", deepInfraChatURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	start := time.Now()
	receivedData := false
	failure := func(what string, err error) error {
		elapsed := time.Since(start).Round(time.Second)
		if stalled.Load() {
			wait := deepInfraIdleTimeout
			if !receivedData {
				wait = deepInfraFirstDataTimeout
			}
			return fmt.Errorf("deepinfra %s: no data received for %s (%s total): %w", what, wait, elapsed, err)
		}
		return fmt.Errorf("deepinfra %s after %s: %w", what, elapsed, err)
	}

	resp, err := deepInfraClient.Do(httpReq)
	if err != nil {
		return "", failure("request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return "", fmt.Errorf("deepinfra returned status %d: %s", resp.StatusCode, string(data))
	}

	var content strings.Builder
	var finishReason string
	var usage *diUsage
	lastProgressLog := start

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		receivedData = true
		idle.Reset(deepInfraIdleTimeout)

		// server-sent events: "data: {...}" lines, blank separators, ": comment" keep-alives
		data, ok := strings.CutPrefix(scanner.Text(), "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			break
		}

		var chunk diStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", fmt.Errorf("decoding deepinfra stream: %w (raw: %s)", err, data)
		}
		if chunk.Error != nil {
			return "", fmt.Errorf("deepinfra error: %s", chunk.Error.Message)
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		for _, choice := range chunk.Choices {
			content.WriteString(choice.Delta.Content)
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				finishReason = *choice.FinishReason
			}
		}

		if time.Since(lastProgressLog) >= deepInfraProgressInterval {
			lastProgressLog = time.Now()
			log.Printf("deepinfra: %s still generating after %s (%d characters so far)",
				req.Model, time.Since(start).Round(time.Second), content.Len())
		}
	}
	if err := scanner.Err(); err != nil {
		return "", failure("stream failed", err)
	}
	if content.Len() == 0 {
		return "", fmt.Errorf("deepinfra returned no content (finish_reason: %q)", finishReason)
	}

	// log how long calls take so the timeouts can be tuned against real recipes
	usageText := "unknown"
	if usage != nil {
		usageText = fmt.Sprintf("%d prompt + %d completion", usage.PromptTokens, usage.CompletionTokens)
	}
	log.Printf("deepinfra: %s took %s (tokens: %s, finish_reason: %q)",
		req.Model, time.Since(start).Round(time.Second), usageText, finishReason)
	if finishReason == "length" {
		log.Printf("deepinfra: %s reply was cut off at the token limit, the recipe JSON is probably incomplete", req.Model)
	}

	return content.String(), nil
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

	// Decode ingredients_used_for_step separately:
   	var envelope struct {
   		Steps                  map[string][]string `json:"steps"`
   		Ingredients            []Ingredient        `json:"ingredients"`
   		IngredientsUsedForStep json.RawMessage     `json:"ingredients_used_for_step"`
   	}
   	if err := json.Unmarshal([]byte(clean), &envelope); err != nil {
   		return parsed, fmt.Errorf("failed to parse recipe json: %w (raw model output: %s)", err, content)
   	}
   	parsed.Steps = envelope.Steps
   	parsed.Ingredients = envelope.Ingredients
   	if len(envelope.IngredientsUsedForStep) > 0 {
   		if err := json.Unmarshal(envelope.IngredientsUsedForStep, &parsed.IngredientsUsedForStep); err != nil {
   			log.Printf("deepinfra: ignoring malformed ingredients_used_for_step: %v", err)
   			parsed.IngredientsUsedForStep = nil
   		}
   	}

	logStepIngredientIssues(parsed)
 
	return parsed, nil
}

func logSchemaDrift(raw string) {
	var generic map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &generic); err != nil {
		return // malformed JSON entirely - the real Unmarshal call will report this
	}
 
	for key := range generic {
		if key != "steps" && key != "ingredients" && key != "ingredients_used_for_step" {
			log.Printf("deepinfra: unexpected top-level field %q in recipe JSON", key)
		}
	}
 
	if rawIngredients, ok := generic["ingredients"]; ok {
		var ingredients []map[string]json.RawMessage
		if err := json.Unmarshal(rawIngredients, &ingredients); err == nil {
			for i, ing := range ingredients {
				for key := range ing {
					if key != "name" && key != "amount" && key != "preparation_notes"  && key != "alt_amount" && key != "component" {
						log.Printf("deepinfra: unexpected field %q in ingredients[%d]", key, i)
					}
				}
			}
		}
	}
}

func logStepIngredientIssues(parsed *RecipeParsed) {
	known := make(map[string]bool, len(parsed.Ingredients))
	for _, ing := range parsed.Ingredients {
		known[strings.ToLower(strings.TrimSpace(ing.Name))] = true
	}

	for component, byStep := range parsed.IngredientsUsedForStep {
		steps, ok := parsed.Steps[component]
		if !ok {
			log.Printf("deepinfra: ingredients_used_for_step has component %q that is not in steps", component)
			continue
		}
		for stepKey, used := range byStep {
			n, err := strconv.Atoi(stepKey)
			if err != nil || n < 1 || n > len(steps) {
				log.Printf("deepinfra: ingredients_used_for_step[%q][%q] is not a valid step number (component has %d steps)", component, stepKey, len(steps))
			}
			for _, u := range used {
				if !known[strings.ToLower(strings.TrimSpace(u.Name))] {
					log.Printf("deepinfra: ingredients_used_for_step[%q][%q] references %q, which is not in the ingredients list", component, stepKey, u.Name)
				}
			}
		}
	}
}

const defaultComponent = "main"

// normalizeIngredientComponents makes every ingredient's component one of the keys in parsed.Steps.
// A missing or unknown component is inferred from ingredients_used_for_step when the ingredient is
// only used in one section, otherwise it falls back to "main".
func normalizeIngredientComponents(parsed *RecipeParsed) {
	normalize := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

	stepKeys := make(map[string]string, len(parsed.Steps))
	for key := range parsed.Steps {
		stepKeys[normalize(key)] = key
	}

	// ingredient name -> components whose steps use it
	usedIn := make(map[string]map[string]bool)
	for component, byStep := range parsed.IngredientsUsedForStep {
		if _, ok := parsed.Steps[component]; !ok {
			continue
		}
		for _, used := range byStep {
			for _, u := range used {
				name := normalize(u.Name)
				if usedIn[name] == nil {
					usedIn[name] = make(map[string]bool)
				}
				usedIn[name][component] = true
			}
		}
	}

	for i := range parsed.Ingredients {
		ing := &parsed.Ingredients[i]
		if key, ok := stepKeys[normalize(ing.Component)]; ok {
			ing.Component = key
			continue
		}

		if ing.Component != "" {
			log.Printf("recipe: ingredient %q has component %q that is not in steps", ing.Name, ing.Component)
		}

		ing.Component = defaultComponent
		if components := usedIn[normalize(ing.Name)]; len(components) == 1 {
			for component := range components {
				ing.Component = component
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
	transcript, err := TranscribeRecipeImages(images)
	if err != nil {
		return &RecipeParsed{}, err
	}
	return extractRecipeJSON(transcript)
}

// TranscribeRecipeImages reads the recipe text from photos with the vision model. This is the slow
// and expensive step of an image job, so the worker saves its result for retries.
func TranscribeRecipeImages(images []string) (string, error) {
	if len(images) == 0 {
		return "", fmt.Errorf("no images provided")
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
		return "", fmt.Errorf("image transcription failed: %w", err)
	}
	if strings.TrimSpace(transcript) == "" {
		return "", fmt.Errorf("image transcription returned no text")
	}

	return transcript, nil
}

var RecipeQueue = make(chan models.RecipeJob, 100)

// jobs that have failed this many times are no longer loaded for retry
const MaxRecipeJobFailures = 3

// usable reports whether a parse result has enough in it to be worth saving/reusing.
func (p *RecipeParsed) usable() bool {
	return p != nil && (len(p.Steps) > 0 || len(p.Ingredients) > 0)
}

// cachedRecipeParse returns the parse result stored on a job from a previous run, or nil if
// there isn't one or it can't be used.
func cachedRecipeParse(jobID int, raw []byte) *RecipeParsed {
	if len(raw) == 0 {
		return nil
	}

	parsed := &RecipeParsed{}
	if err := json.Unmarshal(raw, parsed); err != nil || !parsed.usable() {
		log.Printf("recipe job %d: ignoring stored parsed_json (err: %v)", jobID, err)
		return nil
	}

	return parsed
}

// QueueRecipeJob saves the job to recipe_jobs and then hands it to the worker. Saving first means a
// job still waiting in the queue isn't lost if the server restarts - LoadUnparsedRecipeJobs picks it
// back up on startup.
func QueueRecipeJob(ctx context.Context, job models.RecipeJob) error {
	jobID, err := CreateRecipeJob(ctx, job)
	if err != nil {
		return err
	}
	job.ID = jobID

	RecipeQueue <- job
	return nil
}

func StartRecipeWorker() {
    go func() {
        for job := range RecipeQueue {
            if err := ProcessRecipeJob(context.Background(), job); err != nil {
                log.Printf("Recipe job %q failed: %v", job.Name, err)
                continue
            }
            log.Println("Recipe saved successfully:", job.Name)
        }
    }()
}

// ProcessRecipeJob parses one queued recipe (reusing a stored parse result if there is one) and saves
// it. Failures bump the job's fail_count so it stops being retried after MaxRecipeJobFailures.
func ProcessRecipeJob(ctx context.Context, job models.RecipeJob) error {
    log.Println("Processing recipe:", job.Name)

    jobID := job.ID
    if jobID == 0 {
        var err error
        jobID, err = CreateRecipeJob(ctx, job)
        if err != nil {
            return fmt.Errorf("creating recipe job: %w", err)
        }
    }

    // reuse the model's output from a previous attempt instead of paying for another call
    parsed := cachedRecipeParse(jobID, job.ParsedJSON)
    if parsed != nil {
        log.Println("Reusing stored parse result for recipe:", job.Name)
    } else {
        var err error

        switch job.Type {
        case "image":
            //parsed, err = ParseRecipeImageCall(job.Images)
            transcript := job.Transcript
            if transcript != "" {
                log.Println("Reusing stored transcript for recipe:", job.Name)
            } else if transcript, err = TranscribeRecipeImages(job.Images); err == nil {
                _ = SaveRecipeJobTranscript(ctx, jobID, transcript)
            }
            if err == nil {
                parsed, err = extractRecipeJSON(transcript)
            }
        default: // "text"
            //parsed, err = ParseRecipeCall(job.Text)
            parsed, err = ParseRecipeCallDeepInfra(job.Text)
        }

        if err == nil && !parsed.usable() {
            err = fmt.Errorf("model returned no steps or ingredients")
        }
        if err != nil {
            _ = IncrementRecipeJobFailCount(ctx, jobID)
            return fmt.Errorf("parsing recipe: %w", err)
        }

        if parsedJSON, err := json.Marshal(parsed); err == nil {
            _ = SaveRecipeJobParsedJSON(ctx, jobID, parsedJSON)
        } else {
            log.Println("Error marshaling parsed recipe:", err)
        }
    }

    if err := SaveParsedRecipe(ctx, job.Name, job.User_id, parsed, jobID); err != nil {
        _ = IncrementRecipeJobFailCount(ctx, jobID)
        return fmt.Errorf("saving recipe: %w", err)
    }

    return nil
}

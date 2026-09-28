package lib

// Turning the model's JSON into a RecipeParsed and cleaning it up before it's saved.

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
)

type Ingredient struct {
	Name             string `json:"name"`
	Amount           string `json:"amount"`
	AltAmount        string `json:"alt_amount"`
	PreparationNotes string `json:"preparation_notes"`
	Component        string `json:"component"`
}

type StepIngredient struct {
	Name   string `json:"name"`
	Amount string `json:"amount"`
}

type RecipeParsed struct {
	Steps                  map[string][]string                    `json:"steps"`
	Ingredients            []Ingredient                           `json:"ingredients"`
	IngredientsUsedForStep map[string]map[string][]StepIngredient `json:"ingredients_used_for_step"`
}

func cleanupJSON(raw string) string {
	raw = strings.TrimSpace(raw)

	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")

	return strings.TrimSpace(raw)
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
					if key != "name" && key != "amount" && key != "preparation_notes" && key != "alt_amount" && key != "component" {
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

// keepKnownStepIngredients removes step ingredients that aren't in the recipe's ingredient list -
// the model sometimes lists things the recipe makes along the way ("dough", "1 circle dough",
// "cream filling") - and matches the rest to the ingredient's exact name. Steps and sections left
// empty are removed. Returns how many entries were dropped.
func keepKnownStepIngredients(stepIngredients map[string]map[string][]StepIngredient, ingredientNames []string) int {
	normalize := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

	known := make(map[string]string, len(ingredientNames))
	for _, name := range ingredientNames {
		known[normalize(name)] = name
	}

	dropped := 0
	for component, byStep := range stepIngredients {
		for step, used := range byStep {
			kept := used[:0]
			for _, u := range used {
				name, ok := known[normalize(u.Name)]
				if !ok {
					dropped++
					continue
				}
				u.Name = name
				kept = append(kept, u)
			}

			if len(kept) == 0 {
				delete(byStep, step)
			} else {
				byStep[step] = kept
			}
		}
		if len(byStep) == 0 {
			delete(stepIngredients, component)
		}
	}
	return dropped
}

func ingredientNamesOf(ingredients []Ingredient) []string {
	names := make([]string, len(ingredients))
	for i, ing := range ingredients {
		names[i] = ing.Name
	}
	return names
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

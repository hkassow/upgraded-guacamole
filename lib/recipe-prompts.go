package lib

// System prompts for the DeepInfra models (see deepinfra.go).

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
 
6. Do NOT invent ingredients here. Every "name" must be copied from the "name" of an entry in
   the "ingredients" list - nothing else is allowed.
   - NEVER list things the recipe makes along the way: the dough, dough balls, dough circles,
     scraps, cookie layers, the batter, a mixture, or another component such as "the whiskey
     syrup" or "the cream filling". These are not ingredients - a step that only uses them
     (e.g. "roll out one ball of dough", "brush with the whiskey syrup") gets NO key at all.
 
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

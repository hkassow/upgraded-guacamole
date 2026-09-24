// ---------------------------------------------------------------------------
// Amount combining
//
// Only combines within a compatible unit family (volume-with-volume,
// weight-with-weight, bare count-with-count). Cross-family combining (e.g.
// "2 cups flour" + "200g flour") is NOT attempted - that needs ingredient-
// specific density data we don't have. Unparseable or incompatible amounts
// show up as raw leftover lines instead of being silently dropped or
// wrongly merged.
// ---------------------------------------------------------------------------

// Keyed by parse-ingredient's unitOfMeasureID (with normalizeUOM: true).
// CONFIRMED against the library's own docs: 'cup', 'teaspoon'.
// The rest are inferred from naming convention, NOT independently verified -
// cross-check against `Object.keys(ParseIngredient.unitsOfMeasure)` in the
// console before relying on units beyond cup/teaspoon.
const SHOPPING_LIST_UNIT_INFO = {
	teaspoon: { key: 'volume', toBase: 4.92892 },
	tablespoon: { key: 'volume', toBase: 14.7868 },
	cup: { key: 'volume', toBase: 236.588 },
	'fluid-ounce': { key: 'volume', toBase: 29.5735 },
	milliliter: { key: 'volume', toBase: 1 },
	liter: { key: 'volume', toBase: 1000 },
	gram: { key: 'weight', toBase: 1 },
	kilogram: { key: 'weight', toBase: 1000 },
	ounce: { key: 'weight', toBase: 28.3495 },
	pound: { key: 'weight', toBase: 453.592 },
};

function formatBaseQuantity(key, baseQty) {
	switch (key) {
		case 'weight':
			return baseQty >= 1000 ? `${(baseQty / 1000).toFixed(2)}kg` : `${Math.round(baseQty)}g`;
		case 'volume':
			if (baseQty >= 236.588) return `${(baseQty / 236.588).toFixed(2)} cups`;
			if (baseQty >= 14.7868) return `${(baseQty / 14.7868).toFixed(2)} tbsp`;
			return `${(baseQty / 4.92892).toFixed(2)} tsp`;
		case 'count':
			return `${baseQty}`;
		default:
			return `${baseQty}`;
	}
}

// parseShoppingAmount parses one Amount string - including compound amounts
// like "¾ cup plus 1 tablespoon" - into base-unit totals per family key.
// Returns null if any chunk can't be confidently parsed or uses a unit not
// in SHOPPING_LIST_UNIT_INFO: a partial parse is worse than none, since it
// would silently under-count a real amount.
function parseShoppingAmount(raw) {
	const trimmed = (raw || '').trim();
	if (!trimmed) return null;

	const totals = {};

	for (const chunk of trimmed.split(' plus ')) {
		const results = ParseIngredient.parseIngredient(chunk.trim(), { normalizeUOM: true });
		const parsed = results[0];
		if (!parsed || parsed.quantity == null) return null;

		if (parsed.unitOfMeasureID == null) {
			// No unit recognized -> treat as a bare count, e.g. "3" (for "3 eggs").
			totals.count = (totals.count ?? 0) + parsed.quantity;
			continue;
		}

		const info = SHOPPING_LIST_UNIT_INFO[parsed.unitOfMeasureID];
		if (!info) return null; // unrecognized unit - don't guess, fall back to leftover

		totals[info.key] = (totals[info.key] ?? 0) + parsed.quantity * info.toBase;
	}

	return totals;
}

// buildShoppingList groups ingredients by name (case-insensitive) and sums
// amounts wherever units are compatible. Expects [{ name, amount }, ...].
// Any other fields (alt_amount, preparation_notes, location, etc.) are
// ignored here - only name and amount feed the totals.
function buildShoppingList(ingredients) {
	const order = [];
	const groups = new Map();

	for (const ing of ingredients) {
		const displayName = (ing.name || '').trim();
		const key = displayName.toLowerCase();
		if (!key) continue;

		let group = groups.get(key);
		if (!group) {
			group = { displayName, totals: {}, leftover: [] };
			groups.set(key, group);
			order.push(key);
		}

		const parsed = parseShoppingAmount(ing.amount);
		if (!parsed) {
			if (ing.amount) group.leftover.push(ing.amount);
			continue;
		}

		for (const [k, qty] of Object.entries(parsed)) {
			group.totals[k] = (group.totals[k] ?? 0) + qty;
		}
	}

	return order.map((key) => {
		const group = groups.get(key);
		const lines = Object.entries(group.totals).map(([k, qty]) => formatBaseQuantity(k, qty));
		lines.push(...group.leftover);
		return { name: group.displayName, lines };
	});
}

// ---------------------------------------------------------------------------
// Ingredient collection (bucketing + per-recipe breakdown)
// ---------------------------------------------------------------------------

// buildIngredientCollection flattens the selected recipes' ingredients,
// combines matching ingredients' amounts (via buildShoppingList), and
// buckets the results by location - except "seasoning" category items,
// which are shown as a plain checklist of names with no amount (pantry
// staples where exact combined quantity usually isn't useful).
//
// It also returns collection.byRecipe: the UNMERGED ingredients, grouped
// by recipe title, each with its original amount + prep notes intact -
// since combining loses per-recipe prep context (e.g. "softened" vs "cold
// and cubed" for two different butter entries), this gives you that detail
// back at the bottom of the list instead of trying to guess which note
// belongs on a merged line.
//
// Pure function: no DOM access. recipeIds, recipesById, and ingredientsById
// are plain objects/arrays - pass in your global_recipes / global_ingredients
// directly.
function buildIngredientCollection(recipeIds, recipesById, ingredientsById) {
	const flat = []; // { name, amount, location, category }
	const byRecipe = []; // { title, lines }

	recipeIds.forEach((id) => {
		const recipe = recipesById[id];
		if (!recipe || !recipe.ingredients) return;

		const recipeLines = [];

		recipe.ingredients.forEach((ri) => {
			const fullIngredient = ingredientsById[ri.ingredient_id];
			if (!fullIngredient) return;

			flat.push({
				name: fullIngredient.name,
				amount: ri.amount || '',
				location: fullIngredient.location || 'unspecified',
				category: fullIngredient.category || 'unspecified',
			});

			// Unmerged, per-recipe line - mirrors the original ing_string format.
			let line = ` - ${fullIngredient.name}`;
			if (ri.amount) line += `, ${ri.amount}`;
			if (ri.prep_notes) line += `, ${ri.prep_notes}`;
			recipeLines.push(line);
		});

		// NOTE: assumes recipe.title holds the recipe's name, matching the
		// "title" column from SaveParsedRecipe's INSERT - adjust the property
		// name here if your global_recipes objects use a different key.
		byRecipe.push({ title: recipe.title || `Recipe ${id}`, lines: recipeLines });
	});

	const collection = { seasoning: [] };

	// Seasonings: dedupe by name, no quantity combining - just a checklist.
	const seasoningNames = new Set();
	flat
		.filter((item) => item.category === 'seasoning')
		.forEach((item) => seasoningNames.add(item.name));
	collection.seasoning = [...seasoningNames].map((name) => ` - ${name}`);

	// Everything else: bucket by location, then combine matching ingredients
	// within each bucket so e.g. "sugar" from two recipes becomes one line.
	const byLocation = new Map();
	flat
		.filter((item) => item.category !== 'seasoning')
		.forEach((item) => {
			if (!byLocation.has(item.location)) byLocation.set(item.location, []);
			byLocation.get(item.location).push({ name: item.name, amount: item.amount });
		});

	byLocation.forEach((items, loc) => {
		const combined = buildShoppingList(items);
		collection[loc] = combined.map((entry) => {
			const amountText = entry.lines.length ? `, ${entry.lines.join(' + ')}` : '';
			return ` - ${entry.name}${amountText}`;
		});
	});

	collection.byRecipe = byRecipe;

	return collection;
}

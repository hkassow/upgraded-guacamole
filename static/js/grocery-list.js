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
// in SHOPPING_LIST_UNIT_INFO
function parseShoppingAmount(raw) {
	const trimmed = (raw || '').trim();
	if (!trimmed) return null;

	const totals = {};

	for (const chunk of trimmed.split(' plus ')) {
		const results = ParseIngredient.parseIngredient(chunk.trim() + ' flour', { normalizeUOM: true });
		const parsed = results[0];
		if (!parsed || parsed.quantity == null) return null;

		if (parsed.unitOfMeasureID == null) {
			console.log('no measureID', chunk)
			console.log('checking unitOfMeasure: ', parsed.unitOfMeasure)
			// No unit recognized -> treat as a bare count, e.g. "3" (for "3 eggs").
			totals.count = (totals.count ?? 0) + parsed.quantity;
			continue;
		}

		const info = SHOPPING_LIST_UNIT_INFO[parsed.unitOfMeasureID];
		if (!info) {
			console.warn(
				`shopping-list: unrecognized unit "${parsed.unitOfMeasureID}" (from "${chunk}") - add it to SHOPPING_LIST_UNIT_INFO`
			);
			return null;
		}

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
		const lines = shoppingTotalLines(group.displayName, group.totals);
		lines.push(...group.leftover);
		return { name: group.displayName, lines };
	});
}

// For common ingredients (see conversions.js) cups and grams are merged into one total in the unit
// you'd shop with: "1 cup + 100 g sugar" -> "≈ 300 g", liquids in ml. Everything else is unchanged.
function shoppingTotalLines(name, totals) {
	const combined = IngredientConversions.combineTotals(name, totals);
	if (!combined) {
		return Object.entries(totals).map(([k, qty]) => formatBaseQuantity(k, qty));
	}

	const prefix = combined.converted ? '≈ ' : '';
	return Object.entries(combined.totals).map(([k, qty]) => {
		// exact weights keep their exact grams; only estimates get rounded
		if (k === 'weight') return combined.converted ? prefix + IngredientConversions.formatGrams(qty) : formatBaseQuantity(k, qty);
		if (k === 'volume') return prefix + IngredientConversions.formatMl(qty);
		return formatBaseQuantity(k, qty);
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
			if (ri.preparation_notes) line += `, ${ri.preparation_notes}`;
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

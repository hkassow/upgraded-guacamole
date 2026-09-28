// contains javascript code for main application
// util
// user
// recipe
// dark mode
// modal
//

// -------- on page load --------
window.addEventListener('DOMContentLoaded', () => {
    // business
    fetchRecipes();
    fetchIngredients();


    getCurrentUser();
    
    // ux/ui 
    getSavedColorTheme();
    addWakeLockListener();
    addEventListenerToMenu();
    addEventListenerToRecipeAdd();
});

// -------- global var --------
var making_grocery_list = false;
var global_ingredients = {};
var global_recipes = {};
var wakeLock = null;
var locationFilter = false;
var categoryFilter = false;
var timer = null;
var user = null;
var selectedImageFiles = [];

// -------- util code --------
async function makeRequest() {
    const btn = document.getElementById('requestBtn');
    const responseDiv = document.getElementById('response');

    btn.disabled = true;
    responseDiv.className = 'loading';
    responseDiv.textContent = 'Loading...';

    try {
        const response = await fetch('/hello', {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' }
        });

        const data = await response.text();
        responseDiv.className = 'success';
        responseDiv.textContent = 'Success (' + response.status + '):\n' + data;
    } catch (error) {
	    console.log(error);
        responseDiv.className = 'error';
        responseDiv.textContent = 'Error:\n' + error.message;
    } finally {
        btn.disabled = false;
    }
}

function showToast(message, duration = 10000) { // 10 seconds
    const toast = document.getElementById("toast");
    toast.textContent = message;
    toast.style.opacity = "1";

    setTimeout(() => {
        toast.style.opacity = "0";
    }, duration);
}

// Matches "375°F", "375 °F", "375F", "375 degrees F", "200 degrees Celsius", "180-200°C",
// and unitless "350°" / "350 degrees" (group 3 undefined).
const TEMPERATURE_REGEX = /\b(\d{2,3})(?:\s*(?:-|–|—|to)\s*(\d{2,3}))?(?:(?:\s*[°º]\s*|\s*degrees?\s+|\s*deg\.?\s*)?(fahrenheit|celsius|centigrade|f|c)\b|\s*[°º]|\s+degrees?\b)/gi;
// unitless temps above this are assumed Fahrenheit, at or below it they're left alone ("rotate 180 degrees")
const UNITLESS_FAHRENHEIT_MIN = 250;
// text allowed between two temperatures that are already a conversion pair, e.g. "375°F (190°C)", "190°C/375°F", "375°F or 190°C"
const TEMPERATURE_PAIR_GAP = /^\s*[(\[\/,]?\s*(?:or\s+)?$/i;

function convertTemperature(value, toCelsius) {
    const converted = toCelsius ? (value - 32) * 5 / 9 : value * 9 / 5 + 32;
    // oven temps round to what ovens use (5°C / 25°F), lower temps (internal/candy) stay precise
    const isOvenTemp = toCelsius ? value >= 250 : value >= 120;
    const step = isOvenTemp ? (toCelsius ? 5 : 25) : 1;
    return Math.round(converted / step) * step;
}

// Appends the other temperature unit to a step for display, e.g. "Bake at 375°F" -> "Bake at 375°F (190°C)".
// Also used for the edit box, so saved steps may contain the conversion; already-paired
// temperatures are skipped so it's never added twice.
function addTemperatureConversions(text) {
    const matches = [...text.matchAll(TEMPERATURE_REGEX)];
    if (!matches.length) return text;

    const isUnitless = (m) => !m[3];
    const isFahrenheit = (m) => isUnitless(m) || m[3].toLowerCase().startsWith('f');
    // a unitless temp next to another temp is treated as a pair since we can't tell its unit
    const isPairedWith = (a, b) =>
        a && b && (isUnitless(a) || isUnitless(b) || isFahrenheit(a) !== isFahrenheit(b)) &&
        TEMPERATURE_PAIR_GAP.test(text.slice(a.index + a[0].length, b.index));

    let result = '';
    let lastIndex = 0;
    matches.forEach((m, i) => {
        const end = m.index + m[0].length;
        result += text.slice(lastIndex, end);
        lastIndex = end;

        // recipe already lists both units
        if (isPairedWith(m, matches[i + 1]) || isPairedWith(matches[i - 1], m)) return;
        if (isUnitless(m) && Number(m[1]) <= UNITLESS_FAHRENHEIT_MIN) return;

        const toCelsius = isFahrenheit(m);
        const low = convertTemperature(Number(m[1]), toCelsius);
        const high = m[2] ? convertTemperature(Number(m[2]), toCelsius) : null;
        const unit = toCelsius ? '°C' : '°F';
        result += ` (${high !== null ? `${low}–${high}` : low}${unit})`;
    });
    return result + text.slice(lastIndex);
}

async function enableWakeLock() {
    try {
        if ('wakeLock' in navigator ) {
            wakeLock = await navigator.wakeLock.request('screen');

            // Handle wake lock release (happens on page blur, minimized, etc.)
            wakeLock.addEventListener('release', () => {
                console.log('Screen Wake Lock was released');
            });

            console.log('Screen Wake Lock is active');
        } else {
            console.log('Wake Lock API not supported');
        }
    } catch (err) {
        console.log('Wake Lock error:', err);
    }
}
async function disableWakeLock() {
    try {
        if (wakeLock !== null) {
            await wakeLock.release();
            wakeLock = null;
            console.log("Wake lock disabled");
        } else {
            console.log("Wake lock not active");
        }
    } catch (err) {
        console.error("Error disabling wake lock:", err);
    }
}

function addWakeLockListener() {
    document.addEventListener('visibilitychange', () => {
	const open_modal = document.querySelector(".modal.visible");
    	if (open_modal && document.visibilityState === 'visible') {
            enableWakeLock();
        }
    });

}

async function logCookingToBackEnd(title) {
    const res = await fetch(`/cooking/${title}`);
    console.log(res)
}

function setMakingGroceryList() {
	making_grocery_list = !making_grocery_list
	const groceryBtn = document.getElementById("makeGroceryListBtn");
	const submitBtn = document.getElementById("submitGroceryListBtn");
	const addRecipeBtn = document.getElementById("addRecipeBtn");

	if (!making_grocery_list) {
	    groceryBtn.textContent = "Make Grocery List"
	    groceryBtn.style.backgroundColor = getComputedStyle(document.body)
		.getPropertyValue('--color-accent-mint');
	    submitBtn.style.display = "none";
	    addRecipeBtn.style.display = "inherit";	

	    document.querySelectorAll(".recipe-card.selected")
		.forEach(card => card.classList.remove("selected"));
	} else {
	    groceryBtn.textContent = "Stop"
	    groceryBtn.style.backgroundColor = getComputedStyle(document.body)
                .getPropertyValue('--color-accent-pink');
	    submitBtn.style.display = "inherit";
	    addRecipeBtn.style.display = "none";

	    
	}
}

function submitGroceryList() {
	const recipe_ids = [...document.querySelectorAll(".recipe-card.selected")]
		.map(card => card.dataset.id);
 
	const ingredient_collection = buildIngredientCollection(
		recipe_ids,
		global_recipes,
		global_ingredients,
	);
 
	printIngredientCollection(ingredient_collection);
}

function printIngredientCollection(ingredient_collection) {
	let output = "";

	// everything that isn't byRecipe / originalAmounts is a store location bucket
	const { byRecipe, originalAmounts, ...locationBuckets } = ingredient_collection;
 
	const sortedKeys = Object.keys(locationBuckets).sort();
 
	sortedKeys.forEach(loc => {
		const lines = locationBuckets[loc];
		if (!lines || lines.length === 0) return;
 
		output += `${loc}\n`;
 
		const sortedLines = [...lines].sort((a, b) =>
			a.localeCompare(b, 'en', { sensitivity: 'base' })
		);
		output += sortedLines.join("\n");
		output += "\n\n";
	});
 
	if (byRecipe && byRecipe.length > 0) {
		output += "--- By Recipe (unmerged) ---\n\n";
		byRecipe.forEach(({ title, lines }) => {
			if (!lines || lines.length === 0) return;
			output += `${title}\n`;
			output += lines.join("\n");
			output += "\n\n";
		});
	}

	// every amount as written, to check the merged totals above against
	if (originalAmounts && originalAmounts.length > 0) {
		output += "--- Original amounts by ingredient ---\n\n";
		originalAmounts.forEach(({ name, amounts, total }) => {
			const written = amounts.length ? amounts.join(', ') : 'no amount';
			output += ` - ${name}: ${written}${total ? ` → ${total}` : ''}\n`;
		});
		output += "\n";
	}

	const groceryModal = document.querySelector("#groceryListModal");
	openModal(groceryModal);
	const groceryText = document.querySelector("#groceryListText");
	groceryText.textContent = output.trim();
 

	const btn = document.getElementById("groceryListCopyBtn");
	btn.onclick = () => {
		navigator.clipboard.writeText(output.trim());
	};
}

// -------- user --------
function loginWithGoogle() {
    window.location.href = "/auth/google/login";
}

async function getCurrentUser() {
    const res = await fetch("/auth/me", {
        credentials: "include"
    });

    user = false;
    if (window.location.href.includes('recipes_of')) {
        return null;
    } else if (!res.ok) {
        document.body.classList.add("not-logged-in")
        return null;
    } else {
        user = true;
        document.body.classList.add("logged-in")
    }

    const ret = await res.json();

    document.getElementById("yourFriendCode").textContent = ret.Uuid;
    document.getElementById("yourShareCode").textContent = `https://upgraded-guacamole.com/?recipes_of=${ret.Uuid}`;
}

async function logout() {
    const res = await fetch("/auth/logout", {
        method: "POST",
        credentials: "include"
    });

    if (!res.ok) {
        console.error("Logout failed");
        return;
    }

    user = false;

    document.body.classList.remove("logged-in");
    document.body.classList.add("not-logged-in");

    window.location.reload();
}

async function startFollowing() {
    const value = document.getElementById("displayFollowInput").value;

    try {
        const response = await fetch('/users/follow-new-user', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({friend_code: value.trim()}),
        });

        if (!response.ok) {
            const txt = (await response.text())?.trim();
            if (txt === 'invalid friend code' || txt === 'user does not exist') {
                showToast('Invalid friend code');
            } else if (txt === 'cannot follow yourself') {
                showToast('You cannot follow yourself')
            } else {
                throw new Error(txt);
            }
        } else {
            showToast('Now following user');
            fetchRecipes();
            toggleExpandableSection();
            toggleSettingsMenu();
        }
    } catch (err) {
        showToast('Error following user, please try again later')
        console.error('Error following user:', err);
    }
    
}

// -------- recipe code --------
async function fetchRecipes() {
    const btn = document.getElementById('recipesBtn');
    const responseDiv = document.getElementById('recipesResponse');

    const params = new URLSearchParams(window.location.search);

    const recipesOf = params.get("recipes_of");

    btn.disabled = true;
    responseDiv.className = 'loading';
    responseDiv.textContent = 'Loading recipes...';
    try {
        let url = "/recipes";

        if (recipesOf) {
            url += `?recipes_of=${encodeURIComponent(recipesOf)}`;
        }
        const response = await fetch(url, {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' }
        });

        if (!response.ok) throw new Error('HTTP ' + response.status);

        const recipes = await response.json();

        recipes.sort((a,b) => { return b.title.toLowerCase() > a.title.toLowerCase() ? -1 : 1});
        global_recipes = Object.fromEntries(recipes.map(ing => [ing.id, ing]))

        // modals from the previous render would otherwise pile up in the page
        document.querySelectorAll('.recipe-view-modal').forEach(modal => modal.remove());

        responseDiv.className = '';
        responseDiv.textContent = ''; // clear loading text

        if (recipes.length === 0) {
            responseDiv.textContent = 'No recipes found.';
        } else {
            recipes.forEach(r => {
                const card = document.createElement('div');
                card.className = 'recipe-card';
                const title = document.createElement('h3');
                title.textContent = r.title;
                card.appendChild(title);

                // on your own list, say whose recipe it is when it's from someone you follow
                if (!r.is_mine && r.owner_name && !recipesOf) {
                    const owner = document.createElement('div');
                    owner.className = 'recipe-owner';
                    owner.textContent = `from ${r.owner_name}`;
                    card.appendChild(owner);
                }
                const tags = createRecipeTagList(r.tags);
                if (tags) card.appendChild(tags);

		        card.dataset.id = r.id;
		        responseDiv.appendChild(card);
		        createRecipeModal(card, r)
            });
        }
        renderRecipeFilters(recipes);
    } catch (error) {
	    console.log(error);
        responseDiv.className = 'error';
        responseDiv.textContent = 'Error:\n' + error.message;
        renderRecipeFilters([]);
    } finally {
        btn.disabled = false;
    }
}

async function fetchIngredients() {
	try {
		const response = await fetch('/ingredients', {
			method: 'GET',
			headers: { 'Content-Type': 'application/json' }
		});
		if (!response.ok) throw new Error('HTTP ' + response.status);
		const ingredients = await response.json();
		createTagIngredientsModal(ingredients);
		
		if (ingredients.length) {
		    global_ingredients = Object.fromEntries(ingredients.map(ing => [ing.id, ing]))
		}
	} catch (error) {
		console.log(error);
	}

}

const PARSING_QUEUED_MESSAGE = 'Recipe is being parsed, it should show up in about 5 minutes';

// Runs submit with the form's submit button disabled and showing a spinner, so a double click
// (or pressing Enter twice) can't send the same recipe twice.
async function withSubmitLock(form, loadingLabel, submit) {
    if (form.dataset.submitting === 'true') return;
    form.dataset.submitting = 'true';

    const button = form.querySelector('button[type="submit"]');
    const label = button.textContent;
    button.disabled = true;
    button.classList.add('is-loading');
    button.textContent = loadingLabel;

    try {
        await submit();
    } catch (err) {
        console.error('Error saving recipe:', err);
        showToast('Could not save the recipe, please try again');
    } finally {
        form.dataset.submitting = 'false';
        button.classList.remove('is-loading');
        button.textContent = label;
        button.disabled = false;
    }
}

async function postRecipe(newRecipe) {
    const response = await fetch('/recipes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newRecipe),
    });
    if (!response.ok) throw new Error(`Failed to save recipe (${response.status}): ${await response.text()}`);
}

async function submitRecipeForm(event) {
    event.preventDefault();
    const recipeForm = document.getElementById('aiRecipeForm');
    const newRecipe = {
        name: document.getElementById('recipeName').value.trim(),
        text: document.getElementById('recipeDescription').value.trim(),
        type: 'text'
    };

    await withSubmitLock(recipeForm, 'Saving...', async () => {
        console.log('Submitting new recipe:', newRecipe);
        await postRecipe(newRecipe);

        closeModal('recipeModal');
        recipeForm.reset();
	    showToast(PARSING_QUEUED_MESSAGE);
    });
}

async function submitManualRecipeForm(event) {
    event.preventDefault();
    const recipeForm = document.getElementById('manualRecipeForm');
    const newRecipe = {
        name: document.getElementById('manualRecipeName').value.trim(),
        text: document.getElementById('manualRecipeDescription').value.trim(),
        ingredients: [],
        type: 'manual'
    };

    document.querySelectorAll(".ingredient-row").forEach(row => {
        const amount = row.querySelector(".ingredient-amount").value;
        const name = row.querySelector(".ingredient-name").value;
        const prep_notes = row.querySelector(".ingredient-prep-notes").value;

        newRecipe.ingredients.push({
            amount: amount,
            name: name,
            preparation_notes: prep_notes
        });
    });

    await withSubmitLock(recipeForm, 'Saving...', async () => {
        console.log('Submitting new recipe:', newRecipe);
        await postRecipe(newRecipe);

        fetchRecipes();
        closeModal('recipeModal');
        recipeForm.reset();
	    showToast('Recipe added!');
    });
}

function setImages(files) {
    selectedImageFiles.push(...files);
    renderPreviews();
}

// shows/hides the image form's controls based on whether any photos are selected
function updateImageFormState() {
    const hasImages = selectedImageFiles.length > 0;
    const uploading = document.getElementById('aiImageForm').dataset.submitting === 'true';
    document.getElementById('imageDropPrompt').hidden = hasImages;
    document.getElementById('clearImageBtn').hidden = !hasImages;
    document.getElementById('submitImageBtn').disabled = !hasImages || uploading;
}

function renderPreviews() {
    const grid = document.getElementById('imagePreviewGrid');
    grid.innerHTML = '';
    updateImageFormState();

    selectedImageFiles.forEach((file, index) => {
        const reader = new FileReader();
        reader.onload = () => {
            const wrapper = document.createElement('div');
            wrapper.className = 'image-preview-item';


            const removeBtn = document.createElement('button');
            removeBtn.type = 'button';
            removeBtn.textContent = '×';
            removeBtn.className = 'remove-image-btn';
            removeBtn.onclick = () => {
                selectedImageFiles.splice(index, 1);
                renderPreviews();
            };
            wrapper.appendChild(removeBtn);

            const img = document.createElement('img');
            img.src = reader.result;
            wrapper.appendChild(img);

            grid.appendChild(wrapper);
        };
        reader.readAsDataURL(file);
    });
}

function fileToBase64(file) {
    return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => {
        // strip the "data:image/png;base64," prefix — your Go server expects raw base64
        const base64 = reader.result.split(',')[1];
        resolve(base64);
        };
        reader.onerror = reject;
        reader.readAsDataURL(file);
    });
}

async function submitImageRecipeForm(event) {
    event.preventDefault();
    if (selectedImageFiles.length === 0) return;

    const recipeForm = document.getElementById('aiImageForm');
    const name = document.getElementById('imageRecipeName').value.trim();

    // reading + uploading the photos is the slow part, so it all happens inside the lock
    await withSubmitLock(recipeForm, 'Uploading...', async () => {
        const base64Images = await Promise.all(
            selectedImageFiles.map(file => fileToBase64(file))
        );

        console.log('Submitting new recipe image:', name);
        await postRecipe({
            name: name,
            images: base64Images,
            type: 'image'
        });

        closeModal('recipeModal');
        recipeForm.reset();
        selectedImageFiles = [];
        renderPreviews();
        showToast(PARSING_QUEUED_MESSAGE);
    });

    // the lock re-enables the button; keep it disabled if there are no photos (e.g. after a successful save)
    updateImageFormState();
}

function addIngredient() {
    const addIngredientBtn = document.getElementById("addIngredientBtn");
    const ingredientsList = document.getElementById("ingredientsList");
    const ingredientRow = document.createElement("div");

    ingredientRow.classList.add("ingredient-row");

    ingredientRow.innerHTML = `
        <input
            type="text"
            class="ingredient-amount"
            placeholder="Amount"
        >

        <input
            type="text"
            class="ingredient-name"
            placeholder="Ingredient"
            required
        >
        <input
            type="text"
            class="ingredient-prep-notes"
            placeholder="Prep Notes 'diced'"
        >
        <button type="button" class="remove-ingredient-btn">
            Remove
        </button>
    `;

    ingredientRow
        .querySelector(".remove-ingredient-btn")
        .addEventListener("click", () => {
            ingredientRow.remove();
        });

    ingredientsList.appendChild(ingredientRow);
}

async function deleteRecipe(id, modal) {
    try {
    	const response = await fetch('/recipes', {
            method: 'DELETE',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ recipe_id: id })
        });
        if (!response.ok) throw new Error('Failed to delete recipe');

        fetchRecipes();
        closeModal(modal);
        showToast('Recipe deleted!');
    } catch (err) {
    	console.error('Error deleting recipe:', err);
    }
}

// tags: the full new tag list, or null when the tags weren't changed
async function submitRecipeChanges(id, updated_steps, updated_ingredients, tags, modal) {
    if (!updated_steps?.length && !updated_ingredients?.length && tags === null) {
    	showToast('No changes were made to the recipe');
	return;
    }
    try {
        const body = {recipe_id: id, updated_steps, updated_ingredients};
        if (tags !== null) body.tags = tags;

    	const response = await fetch('/recipes', {
	    method: 'PATCH',
	    headers: {'Content-Type': 'application/json' },
	    body: JSON.stringify(body)
	});
	if (!response.ok) {
	    const message = (await response.text()).trim();
	    // 400s explain what's wrong (e.g. a tag that's too long), show that to the user
	    showToast(response.status === 400 && message ? message : 'Could not save your changes, please try again');
	    throw new Error(`Failed to edit recipe (${response.status}): ${message}`);
	}

	fetchRecipes();
	closeModal(modal);
	showToast('Recipe updated!');
    } catch (err) {
    	console.error('Error editing recipe:', err);
    }
}


// -------- dark mode code --------
function toggleDarkMode() {
    document.body.classList.toggle('dark');
    const isDark = document.body.classList.contains('dark');
    localStorage.setItem('theme', isDark ? 'dark' : 'light');
}

function getSavedColorTheme() {
    document.body.classList.add('notransition');
    const savedTheme = localStorage.getItem('theme');

    if (savedTheme) {
        // Apply the saved theme
        if (savedTheme === 'dark') {
            document.body.classList.add('dark');
        }
    } else {
        // Detect system preference
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;

        if (prefersDark) {
            document.body.classList.add('dark');
            localStorage.setItem('theme', 'dark');
        } else {
            localStorage.setItem('theme', 'light');
        }
    }
    requestAnimationFrame(() => {
        document.body.classList.remove('notransition');
	document.body.classList.add('page-loaded');
    });
}

// -------- modal code --------
function openModal(modalElement) {
    const modal = typeof modalElement === 'string'
        ? document.getElementById(modalElement)
        : modalElement;

    modal.style.display = 'flex';
    modal.classList.add('visible');

    enableWakeLock();
}

function closeModal(modalElement) {
    const modal = typeof modalElement === 'string'
        ? document.getElementById(modalElement)
        : modalElement;

    modal.classList.remove('visible');
    setTimeout(() => {
        modal.style.display = 'none';
    }, 200);

    if (timer){
        clearTimeout(timer);
    }

    disableWakeLock();
}

function handleModalBackgroundClick(event, modalElement) {
    if (event.target === modalElement) {
        closeModal(modalElement);
    }
}

// suggested in the tag editor alongside any tag already used on a visible recipe
const DEFAULT_TAGS = ['vegetarian', 'vegan', 'breakfast', 'lunch', 'dinner', 'dessert', 'baking', 'bread', 'snack', 'side'];
const MAX_TAG_LENGTH = 30; // matches the server
const MAX_TAGS_PER_RECIPE = 20;

// same rules as lib.NormalizeTags on the server: "  #Quick   Dinner " -> "quick dinner"
function normalizeTag(tag) {
    return tag.trim().replace(/^#/, '').replace(/\s+/g, ' ').toLowerCase();
}

// small read-only chips, or null when there are no tags
function createRecipeTagList(tags) {
    if (!tags || !tags.length) return null;
    const list = document.createElement('ul');
    list.className = 'recipe-tags';
    list.setAttribute('aria-label', 'Tags');
    tags.forEach(tag => {
        const item = document.createElement('li');
        item.textContent = tag;
        list.appendChild(item);
    });
    return list;
}

let tagEditorCount = 0;

// Editable tag list for the edit form: chips with a remove button, and an input that adds a tag
// on Enter or comma (with suggestions). getTags() also picks up anything still typed in the input.
function createTagEditor(initialTags) {
    let tags = [...initialTags];

    const wrapper = document.createElement('div');
    wrapper.className = 'tag-editor';

    const list = document.createElement('ul');
    list.className = 'recipe-tags';
    list.setAttribute('aria-label', 'Tags');

    const suggestions = document.createElement('datalist');
    suggestions.id = `tagSuggestions${++tagEditorCount}`;

    const input = document.createElement('input');
    input.type = 'text';
    input.placeholder = 'Add a tag, e.g. dinner';
    input.setAttribute('aria-label', 'Add a tag');
    input.maxLength = MAX_TAG_LENGTH;
    input.setAttribute('list', suggestions.id);

    function render() {
        list.innerHTML = '';
        tags.forEach(tag => {
            const item = document.createElement('li');
            item.textContent = tag;
            const remove = document.createElement('button');
            remove.type = 'button';
            remove.className = 'tag-remove';
            remove.textContent = '×';
            remove.setAttribute('aria-label', `Remove tag ${tag}`);
            remove.addEventListener('click', () => {
                tags = tags.filter(t => t !== tag);
                render();
            });
            item.appendChild(remove);
            list.appendChild(item);
        });

        const known = new Set(DEFAULT_TAGS);
        Object.values(global_recipes).forEach(r => (r.tags || []).forEach(t => known.add(t)));
        suggestions.innerHTML = '';
        [...known].filter(t => !tags.includes(t)).sort().forEach(t => {
            const option = document.createElement('option');
            option.value = t;
            suggestions.appendChild(option);
        });
    }

    function addFromInput() {
        input.value.split(',').map(normalizeTag).filter(Boolean).forEach(tag => {
            if (!tags.includes(tag) && tags.length < MAX_TAGS_PER_RECIPE) tags.push(tag);
        });
        input.value = '';
        render();
    }

    input.addEventListener('keydown', (event) => {
        if (event.key === 'Enter' || event.key === ',') {
            event.preventDefault();
            addFromInput();
        }
    });
    // picking a suggestion from the list adds it straight away
    input.addEventListener('input', (event) => {
        if (event.inputType === 'insertReplacementText' || event.inputType === undefined) addFromInput();
    });

    render();
    wrapper.append(list, input, suggestions);

    return {
        element: wrapper,
        getTags() {
            if (input.value.trim()) addFromInput();
            return [...tags];
        },
    };
}

// recipes saved before sections existed have no component, treat them as main
function ingredientComponent(ing) {
    return ing.component || 'main';
}

// recipe sections in display order: main first, then the other step sections, then any
// section only an ingredient uses
function recipeComponents(recipe) {
    const components = ['main'];
    const add = (component) => {
        if (!components.includes(component)) components.push(component);
    };
    Object.keys(recipe.steps || {}).forEach(add);
    recipe.ingredients.forEach(ing => add(ingredientComponent(ing)));
    return components;
}

function capitalizeComponent(component) {
    const name = component.replace(/_/g, ' ');
    return name.charAt(0).toUpperCase() + name.slice(1);
}

// "½ cup" + "granulated sugar" -> "½ cup granulated sugar", "remaining" -> "remaining granulated sugar"
function formatStepIngredient(ing) {
    return [ing.amount, ing.name].map(s => (s || '').trim()).filter(Boolean).join(' ');
}

// one instruction step, with the ingredients it uses listed underneath (from step_ingredients,
// which is keyed by section and 1-based step number)
function createStepItem(recipe, component, index, text) {
    const li = document.createElement('li');
    const stepText = document.createElement('div');
    stepText.textContent = text;
    li.appendChild(stepText);

    const used = (recipe.step_ingredients?.[component]?.[String(index + 1)] || [])
        .map(formatStepIngredient)
        .filter(Boolean);
    if (used.length) {
        const list = document.createElement('ul');
        list.className = 'step-ingredients';
        list.setAttribute('aria-label', 'Ingredients for this step');
        used.forEach(text => {
            const item = document.createElement('li');
            item.textContent = text;
            list.appendChild(item);
        });
        li.appendChild(list);
    }
    return li;
}

function createRecipeModal(card, recipe) {
    const modal = document.createElement('div');
    modal.className = 'modal recipe-view-modal';
    modal.addEventListener('click', (e) => {
        if (e.target === modal) closeModal(modal);
    });

    // only the owner can edit (the server also enforces this)
    modal.innerHTML = `
        <div class="modal-content recipe-modal">
            <span class="close">&times;</span>
            <h2 class="recipe-modal-title" style="text-transform: capitalize;"></h2>
	        ${recipe.is_mine ? `
                <div class="button-container">
                    <br>
                    <button class="edit-recipe-btn" data-mode="view">Edit Recipe</button>
                    <button class="delete-recipe-btn" hidden>Delete Recipe</button>
                </div>`
            : '' }
	        <div class="viewRecipe">
            	<div class="viewTags"></div>
            	<h3>Ingredients</h3>
            	<div class="ingredientsList"></div>
           	<div class="stepsContainer"></div>
	    </div>
	    <div class="editRecipe" style="display: none;">
	        <h3>Tags</h3>
	        <div class="editTags"></div>
	        <h3>Edit Ingredients</h3>
		<div class="editIngredientsList"></div>
		<div class="editStepsContainer">
		    <h3>Instructions</h3>
		    <label>Main steps</label>
		</div>
		<br/>
		<button class="submit-edit-recipe-btn"> Submit Changes</button>
	    </div>
        </div>
    `;

    document.body.appendChild(modal);

    // set as text, not HTML, so a title can't inject markup
    modal.querySelector('.recipe-modal-title').textContent = recipe.title;
    const viewTags = createRecipeTagList(recipe.tags);
    if (viewTags) modal.querySelector('.viewTags').appendChild(viewTags);

    const tagEditor = createTagEditor(recipe.tags || []);
    modal.querySelector('.editTags').appendChild(tagEditor.element);

    modal.querySelector('.close').addEventListener('click', () => closeModal(modal));
    // open recipe click
    card.addEventListener('click', () => {
        if (making_grocery_list) {
            card.classList.toggle("selected");
        } else {
            openModal(modal);
            // if modal is open for 2 minutes assume cooking
            timer = setTimeout(() => logCookingToBackEnd(recipe.title), 120000)
        }
    });
    if (recipe.is_mine) {
    // edit recipe-modal click
        const deleteRecipeBtn = modal.querySelector(".delete-recipe-btn");

        modal.querySelector(".edit-recipe-btn").addEventListener("click", (event) => {
            const viewSection = modal.querySelector(".viewRecipe");
            const editSection = modal.querySelector(".editRecipe");
            const button = event.target;

            const isEditing = button.dataset.mode === "editing";

            if (!isEditing) {
                button.innerText = "View Recipe";
                button.dataset.mode = "editing";

                deleteRecipeBtn.hidden = false;

                viewSection.style.display = "none";
                editSection.style.display = "block";
            } else {
                button.innerText = "Edit Recipe";
                button.dataset.mode = "view";

                deleteRecipeBtn.hidden = true;

                viewSection.style.display = "block";
                editSection.style.display = "none";
            }
        });
        modal.querySelector(".delete-recipe-btn").addEventListener("click", (event) => {
            if (confirm(`Are you sure you want to delete ${recipe.title}`)) {
                deleteRecipe(recipe.id, modal);
            }
        })
    }
    
    // edit recipe confirm
    modal.querySelector(".submit-edit-recipe-btn").addEventListener("click", (event) => {
	const originals = recipe.ingredients;
	const updated_ingredients = recipe.ingredients.map((ing, idx) => {
            // section picker only exists when the recipe has more than one section
            const componentSelect = modal.querySelector(`.componentInput[data-index="${idx}"]`);
            return {
		...ing,
		idx: idx,
                name: modal.querySelector(`.nameInput[data-index="${idx}"]`).value.trim(),
                amount: modal.querySelector(`.amountInput[data-index="${idx}"]`).value.trim(),
                preparation_notes: modal.querySelector(`.prepInput[data-index="${idx}"]`).value.trim(),
                component: componentSelect ? componentSelect.value : ingredientComponent(ing)
            };
        }).filter(ing => ing.name != originals[ing.idx].name || ing.amount != originals[ing.idx].amount || ing.preparation_notes != originals[ing.idx].preparation_notes || ing.component != ingredientComponent(originals[ing.idx])).map(({idx, ...keepAttrs}) => keepAttrs);

	
	const updated_instructions = Array.from(modal.querySelectorAll('.edit-instructions'), (ing) => {
	    return {
		original_steps: ing.dataset.originalInstructions.trim(),
    		new_steps: ing.value.trim(),
	        step_name: ing.dataset.name
            }
	}).filter(ing => ing.original_steps !== ing.new_steps);

	// tags come back sorted from the server, so compare sorted
	const newTags = tagEditor.getTags();
	const tagsChanged = [...newTags].sort().join('\n') !== [...(recipe.tags || [])].sort().join('\n');

	submitRecipeChanges(recipe.id, updated_instructions, updated_ingredients, tagsChanged ? newTags : null, modal);
	
	// for updated ingredients if only the amount or prep notes changed we dont need a new ingredient x recipe relation
	// if name changes find ingredient or create and then change the linked keys

    });

    // Populate ingredients, grouped by recipe section (only shows section headings when there's more than one)
    const ingredientsList = modal.querySelector('.ingredientsList');
    const components = recipeComponents(recipe);
    const showSectionHeadings = components.length > 1;
    components.forEach(component => {
        const sectionIngredients = recipe.ingredients.filter(ing => ingredientComponent(ing) === component);
        if (!sectionIngredients.length) return;

        if (showSectionHeadings && component !== 'main') {
            const heading = document.createElement('h4');
            heading.className = 'ingredient-section-heading';
            heading.textContent = capitalizeComponent(component);
            ingredientsList.appendChild(heading);
        }

        const ul = document.createElement('ul');
        sectionIngredients.forEach(ing => {
            const li = document.createElement('li');
            // the recipe's own alternative amount wins; otherwise estimate grams/cups/ml for common ingredients
            const altAmount = ing.alt_amount || IngredientConversions.alternativeAmount(ing.name, ing.amount);
            li.textContent = `${ing.amount} ${altAmount ? '(' + altAmount + ') ' : ''}${ing.name} ${ing.preparation_notes || ''}`.trim();
            ul.appendChild(li);
        });
        ingredientsList.appendChild(ul);
    });

    // Populatae edit ingredients
    const editIngredientsList = modal.querySelector('.editIngredientsList');
    // header and rows share a grid so the column names line up with the inputs
    const rowClass = showSectionHeadings ? 'edit-ingredient-row with-section' : 'edit-ingredient-row';
    if (recipe.ingredients.length) {
        const header = document.createElement('div');
        header.className = `${rowClass} edit-ingredient-header`;
        header.setAttribute('aria-hidden', 'true'); // each input has its own aria-label
        const columns = ['Ingredient', 'Amount', 'Prep notes'];
        if (showSectionHeadings) columns.push('Section');
        columns.forEach(label => {
            const cell = document.createElement('span');
            cell.textContent = label;
            header.appendChild(cell);
        });
        editIngredientsList.appendChild(header);
    }

    recipe.ingredients.forEach((ing, idx) => {
    	const row = document.createElement('div');
        row.className = rowClass;
	    row.innerHTML = `
            <input type="text" class="nameInput" data-index="${idx}" placeholder="Name" aria-label="Ingredient ${idx + 1} name">
            <input type="text" class="amountInput" data-index="${idx}" placeholder="Amount" aria-label="Ingredient ${idx + 1} amount">
            <input type="text" class="prepInput" data-index="${idx}" placeholder="Prep Notes" aria-label="Ingredient ${idx + 1} prep notes">
	    `;
        // set as values, not HTML attributes, so quotes etc. in a name can't break out of the input
        row.querySelector('.nameInput').value = ing.name || '';
        row.querySelector('.amountInput').value = ing.amount || '';
        row.querySelector('.prepInput').value = ing.preparation_notes || '';

        if (showSectionHeadings) {
            const select = document.createElement('select');
            select.className = 'componentInput';
            select.dataset.index = idx;
            select.title = 'Recipe section';
            select.setAttribute('aria-label', `Ingredient ${idx + 1} section`);
            components.forEach(component => {
                const option = document.createElement('option');
                option.value = component;
                option.textContent = capitalizeComponent(component);
                select.appendChild(option);
            });
            select.value = ingredientComponent(ing);
            row.appendChild(select);
        }

	    editIngredientsList.appendChild(row);
    });
    

    // Populate steps
    const stepsContainer = modal.querySelector('.stepsContainer');
    const editStepsContainer = modal.querySelector('.editStepsContainer');
    if (recipe.steps.main) {
        const mainSection = document.createElement('div');
        const mainTitle = document.createElement('h3');
        mainTitle.textContent = 'Instructions';
        mainSection.appendChild(mainTitle);
    
	    let fullInstructions = '';
	    let rows = 1;

        const mainOl = document.createElement('ol');
        recipe.steps.main.forEach((step, index) => {
            // edit box shows the same converted text so it matches what's displayed
            const displayStep = addTemperatureConversions(step);
            mainOl.appendChild(createStepItem(recipe, 'main', index, displayStep));
	        if (fullInstructions) {
	    	    fullInstructions += '\n\n';
	        }
	        fullInstructions += displayStep
            rows += 1;
        });
        mainSection.appendChild(mainOl);
        stepsContainer.appendChild(mainSection);
	
	    const editMain = document.createElement('textarea');
	    editMain.textContent = fullInstructions;
	    editMain.className = 'edit-instructions';
	    editMain.dataset.originalInstructions = fullInstructions;
	    editMain.dataset.name = 'main'
	    editMain.rows = rows * 3;
	    editStepsContainer.append(editMain);
    }
    
    // Add other subcomponents
    Object.entries(recipe.steps).forEach(([component, steps]) => {
        if (component === 'main') return;
    
        const section = document.createElement('div');
        const title = document.createElement('h3');
        title.textContent = capitalizeComponent(component);
        section.appendChild(title);

        let fullInstructions = '';
        let rows = 1;

        const ol = document.createElement('ol');
        steps.forEach((step, index) => {
            const displayStep = addTemperatureConversions(step);
            ol.appendChild(createStepItem(recipe, component, index, displayStep));
            if (fullInstructions) {
                fullInstructions += '\n\n';
            }
            fullInstructions += displayStep
            rows += 1;
    });
    section.appendChild(ol);
    stepsContainer.appendChild(section);
	
	const label = document.createElement('label');
	label.textContent = capitalizeComponent(component);
	const editBox = document.createElement('textarea');
	editBox.textContent = fullInstructions;
	editBox.className = 'edit-instructions';
	editBox.dataset.originalInstructions = fullInstructions;
	editBox.dataset.name = component;
	editBox.rows = rows * 3;
	editStepsContainer.append(label);
	editStepsContainer.append(editBox);
    });
}

function createTagIngredientsModal(ingredients) {
    // Remove existing modal if present
    const existing = document.getElementById("tagIngredientsModal");
    if (existing) existing.remove();

    const modal = document.createElement("div");
    modal.id = "tagIngredientsModal";
    modal.className = "modal";
    modal.addEventListener('click', (e) => {
        if (e.target === modal) closeModal(modal);
    });

    modal.innerHTML = `
        <div class="modal-content" style="max-width: 900px; width: 80%;">
            <span class="close">&times;</span>
            <h2 style="margin-bottom: 0px; text-align: center;">Tag Ingredients</h2>
	    <div id="filter-list">
	        <label class="filter-toggle">
                    <input onchange="changeFilter(true,false)" type="checkbox" id="filter-no-category">
                    Missing Category Only
                </label>
                <label class="filter-toggle">
                    <input onchange="changeFilter(false,true)" type="checkbox" id="filter-no-location">
                    Missing Location Only
                </label>
	    </div>
            <div id="ingredientTagList"></div>

            <div style="margin-top: 20px; text-align: right;">
                <button id="saveIngredientTagsBtn">Save Tags</button>
            </div>
        </div>
    `;

    document.body.appendChild(modal);

    // Close button
    modal.querySelector(".close").addEventListener("click", () => closeModal(modal));

    // Fill list with ingredient rows
    const listContainer = modal.querySelector("#ingredientTagList");
    
    addIngredientRows(listContainer, ingredients);

    // Save handler
    modal.querySelector("#saveIngredientTagsBtn").addEventListener("click", async () => {
        const updated = ingredients.map((ing, idx) => {
            return {
                ...ing,
                category: modal.querySelector(`.catInput[data-index="${ing.id}"]`)?.value?.trim(),
                location: modal.querySelector(`.locInput[data-index="${ing.id}"]`)?.value?.trim(),
	            season: modal.querySelector(`.seasonInput[data-index="${ing.id}"]`)?.value?.trim()
            };
        }).filter(ing => ing.category || ing.location || ing.season);

	    if (updated.length === 0) {
		    showToast("No changes to save");
		    return;
	    }
	    try {
            const resp = await fetch("/ingredients", {
        	    method: "POST",
        	    headers: { "Content-Type": "application/json" },
        	    body: JSON.stringify(updated),
            });

            if (!resp.ok) {
                alert("Failed to save ingredients");
                return;
            }
		
            showToast("Ingredients saved successfully!");
		    fetchIngredients();
            closeModal(modal);
		    locationFilter = false;
		    categoryFilter = false;

        } catch (err) {
        	console.error("Failed to save ingredients:", err);
        	showToast("Error saving ingredients.");
        }
    });

}
function changeFilter(categoryBool, locationBool) {
    if (categoryBool) {
    	categoryFilter = !categoryFilter;
	
    } else if (locationBool) {
    	locationFilter = !locationFilter;
    } else {
    	return;
    }
    const listContainer = document.querySelector("#ingredientTagList");
    
    while (listContainer.firstChild) {
	    listContainer.removeChild(listContainer.lastChild)
    }
    addIngredientRows(listContainer, Object.values(global_ingredients));
}

function addIngredientRows(container, ingredients) {
    ingredients.sort((a,b) => {return a.name.toLowerCase() > b.name.toLowerCase() ? 1 : -1});
    ingredients.forEach((ing, idx) => {
	if ((!locationFilter || !ing.location) && (!categoryFilter || !ing.category)) {
            const row = document.createElement("div");
            row.style.cssText = `
                display:flex;
                align-items:center;
                gap:10px;
                padding:6px 0;
            `;

            // ingredient names come from every user's recipes, so they're set as text, never HTML
            row.innerHTML = `
                <div class="ingredientTagName" style="width: 200px;"></div>
                <input type="text" class="catInput" data-index="${ing.id}" placeholder="Category">
                <input type="text" class="locInput" data-index="${ing.id}" placeholder="Location">
                <input type="text" class="seasonInput" data-index="${ing.id}" placeholder="Season">
            `;
            row.querySelector('.ingredientTagName').textContent = ing.name;
            row.querySelector('.catInput').value = ing.category || '';
            row.querySelector('.locInput').value = ing.location || '';
            row.querySelector('.seasonInput').value = ing.season || '';

            container.appendChild(row);
	}
    });
}


function toggleSettingsMenu(event) {
    event?.stopPropagation();

    const menu = document.getElementById("settingsMenu");
    menu.classList.toggle("visible");
    document
        .getElementById("expandableSection")
        ?.classList.remove("visible")
}

function addEventListenerToMenu() {
    document.addEventListener("click", (event) => {
        const menu = document.getElementById("settingsMenu");

        // Ignore clicks inside the menu
        if (menu && menu.contains(event.target)) {
            return;
        }

        menu?.classList.remove("visible");

        document
            .getElementById("expandableSection")
            ?.classList.remove("visible")
    });

    const copyBtns = document.querySelectorAll(".copy-btn");

    copyBtns.forEach(copyBtn => {
        copyBtn.addEventListener("click", async () => {
            const text = copyBtn.dataset.copy;

            const value = copyBtn.querySelector(".friend-code-value");

            if (!value) return;

            await navigator.clipboard.writeText(value.textContent.trim());

            console.log("Copied:", value.textContent.trim());
        });
    });

    document.getElementById("logoutBtn").addEventListener("click", logout);
}

function addEventListenerToRecipeAdd() {
    // manual add recipe setup
    addIngredientBtn.addEventListener("click", addIngredient);
    addIngredient();

    const modeForms = {
        text: document.getElementById('aiRecipeForm'),
        image: document.getElementById('aiImageForm'),
        manual: document.getElementById('manualRecipeForm'),
    };

    document.querySelectorAll('.mode-tab').forEach(tab => {
        tab.addEventListener('click', () => switchRecipeMode(tab.dataset.mode));
    });

    // --- Image handling ---

    const dropZone = document.getElementById('imageDropZone');
    const fileInput = document.getElementById('recipeImageInput');
    const preview = document.getElementById('imagePreview');
    const dropPrompt = document.getElementById('imageDropPrompt');
    const clearBtn = document.getElementById('clearImageBtn');
    const submitImageBtn = document.getElementById('submitImageBtn');

    const nameFieldIds = {
        text: 'recipeName',
        image: 'imageRecipeName',
        manual: 'manualRecipeName',
    };

    function switchRecipeMode(mode) {
        // keep current name(title)
        const currentModeEntry = Object.entries(modeForms).find(([, form]) => !form.hidden);
        let carriedName = '';
        if (currentModeEntry) {
            const [currentMode] = currentModeEntry;
            const currentNameField = document.getElementById(nameFieldIds[currentMode]);
            if (currentNameField) carriedName = currentNameField.value.trim();
        }

        Object.entries(modeForms).forEach(([key, form]) => {
            form.hidden = key !== mode;
        });

        document.querySelectorAll('.mode-tab').forEach(tab => {
            tab.classList.toggle('active', tab.dataset.mode === mode);
        });

        if (carriedName) {
            const newNameField = document.getElementById(nameFieldIds[mode]);
            if (newNameField) newNameField.value = carriedName;
        }

        if (mode === 'image') {
            dropZone.focus(); // so paste events land here
        }
    }


    dropZone.addEventListener('click', () => fileInput.click());

    dropZone.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') fileInput.click();
    });

    fileInput.addEventListener('change', () => {
        if (fileInput.files.length) setImages([...fileInput.files]);
        fileInput.value = ''; // reset so selecting the same file again still fires 'change'
    });


    dropZone.addEventListener('dragover', (e) => {
        e.preventDefault();
        dropZone.classList.add('drag-over');
    });

    dropZone.addEventListener('dragleave', () => {
        dropZone.classList.remove('drag-over');
    });

    dropZone.addEventListener('drop', (e) => {
        e.preventDefault();
        dropZone.classList.remove('drag-over');
        const files = [...e.dataTransfer.files].filter(f => f.type.startsWith('image/'));
        if (files.length) setImages(files);
    });

    document.addEventListener('paste', (e) => {
        if (document.getElementById('aiImageForm').hidden) return;
        if (!e.clipboardData) return;

        const imageFiles = [...e.clipboardData.items]
            .filter(i => i.type.startsWith('image/'))
            .map(i => i.getAsFile())
            .filter(Boolean);
        if (imageFiles.length) {
            e.preventDefault();
            setImages(imageFiles);
        }
    });


    clearBtn.addEventListener('click', () => {
        selectedImageFiles = [];
        renderPreviews();
    });
}

function toggleExpandableSection(event) {
    event?.stopPropagation();

    document
        .getElementById("expandableSection")
        ?.classList.toggle("visible");
    
    document.getElementById("displayFollowInput").value = ""
}

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
	
	const ingredient_collection = {'seasoning': []};

	recipe_ids.forEach(id => {
	    const recipe = global_recipes[id];
            if (!recipe || !recipe.ingredients) return;
	    recipe.ingredients.forEach(ri => {
            	const fullIngredient = global_ingredients[ri.ingredient_id];
            	if (fullIngredient) {
		    loc = fullIngredient.location;
	            cat = fullIngredient.category || 'unspecified';
		    if (!(loc in ingredient_collection)) {
		    	ingredient_collection[loc] = {}
		    } 
	            let ing_string = ' - ' + fullIngredient.name;
	            ing_string += ri.amount? `, ${ri.amount}` : '';
		    ing_string += ri.prep_notes? `, ${ri.prep_notes}` : '';
		    
	            if (cat === 'seasoning') {
			   	ingredient_collection[cat].push(' - ' + fullIngredient.name);
		    } else {
			if (!(cat in ingredient_collection[loc])) {
				ingredient_collection[loc][cat] = []
			}
               	    	ingredient_collection[loc][cat].push(ing_string);
		    }
            	}
       	    });

	})
	printIngredientCollection(ingredient_collection);
}

function printIngredientCollection(ingredient_collection) {
    let output = "";

    // Sort keys
    const sortedKeys = Object.keys(ingredient_collection).sort();

    sortedKeys.forEach(loc => {
	output += `${loc}\n`;

        const categories = ingredient_collection[loc];

        // sort categories alphabetically
        const sortedCats = Object.keys(categories).sort();
	
	if (loc === 'seasoning') {
		const sorted = categories.sort();
		output += sorted.join("\n") + "\n\n";
	}
        sortedCats.forEach(cat => {
	    if (loc !== 'seasoning') {
            	// Sort each category's ingredient list alphabetically
            	categories[cat].sort((a, b) =>
                	a.localeCompare(b, 'en', { sensitivity: 'base' })
            	);

            	output += categories[cat].join("\n") + "\n";
	    }
        });
	output += "\n\n";
    });
    const groceryModal = document.querySelector("#groceryListModal");
    openModal(groceryModal);
    const groceryText = document.querySelector("#groceryListText");
    groceryText.textContent = output.trim();
    const btn = document.getElementById("groceryListCopyBtn");
    btn.addEventListener("click", () => {
        navigator.clipboard.writeText(output.trim());
    });
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
	
        if (recipes.length) {
		    recipes.sort((a,b) => { return b.title.toLowerCase() > a.title.toLowerCase() ? -1 : 1});
        	global_recipes = Object.fromEntries(recipes.map(ing => [ing.id, ing]))
        }

        responseDiv.className = '';
        responseDiv.textContent = ''; // clear loading text

        if (recipes.length === 0) {
            responseDiv.textContent = 'No recipes found.';
        } else {
            recipes.forEach(r => {
                const card = document.createElement('div');
                card.className = 'recipe-card';
                card.innerHTML = `
                    <h3>${r.title}</h3>
                `;
		        card.dataset.id = r.id;
		        responseDiv.appendChild(card);
		        createRecipeModal(card, r)
            });
        }
    } catch (error) {
	    console.log(error);
        responseDiv.className = 'error';
        responseDiv.textContent = 'Error:\n' + error.message;
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

async function submitRecipeForm(event) {
    event.preventDefault();
    const recipeForm = document.getElementById('recipeForm');
    const newRecipe = {
        name: recipeForm.recipeName.value.trim(),
	    text: recipeForm.recipeDescription.value.trim()    
    };

    try {
        console.log('Submitting new recipe:', newRecipe);
        const response = await fetch('/recipes', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(newRecipe),
        });

        if (!response.ok) throw new Error('Failed to save recipe');

        closeModal('recipeModal');
        recipeForm.reset();
	
	    showToast('Recipe queued to be parsed, please check back later');
    } catch (err) {
        console.error('Error saving recipe:', err);
    }
}

async function submitManualRecipeForm(event) {
    event.preventDefault();
    const recipeForm = document.getElementById('manualRecipeForm');
    const newRecipe = {
        name: recipeForm.recipeName.value.trim(),
	    text: recipeForm.recipeDescription.value.trim(),
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

    try {
        console.log('Submitting new recipe:', newRecipe);
        const response = await fetch('/recipes', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(newRecipe),
        });

        if (!response.ok) throw new Error('Failed to save recipe');

        closeModal('recipeModal');
        recipeForm.reset();
	
	    showToast('Recipe added!');
    } catch (err) {
        console.error('Error saving recipe:', err);
    }
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

async function submitRecipeChanges(id, updated_steps, updated_ingredients, modal) {
    if (!updated_steps?.length && !updated_ingredients?.length) {
    	showToast('No changes were made to the recipe');
	return;
    }
    try {
    	const response = await fetch('/recipes', {
	    method: 'PATCH',
	    headers: {'Content-Type': 'application/json' },
	    body: JSON.stringify({recipe_id: id, updated_steps, updated_ingredients})
	});
	if (!response.ok) throw new Error('Failed to edit recipe');

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

function createRecipeModal(card, recipe) {
    const modal = document.createElement('div');
    modal.className = 'modal';
    modal.addEventListener('click', (e) => {
        if (e.target === modal) closeModal(modal);
    });

    modal.innerHTML = `
        <div class="modal-content recipe-modal">
            <span class="close">&times;</span>
            <h2 style="text-transform: capitalize;">${recipe.title}</h2>
	        ${user? `
                <div class="button-container">
                    <br>
                    <button class="edit-recipe-btn" data-mode="view">Edit Recipe</button>
                    <button class="delete-recipe-btn" hidden>Delete Recipe</button>
                </div>` 
            : '' }
	        <div class="viewRecipe">
            	<h3>Ingredients</h3>
            	<ul class="ingredientsList"></ul>
           	<div class="stepsContainer"></div>
	    </div>
	    <div class="editRecipe" style="display: none;">
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
    if (user) {
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
            return {
		...ing,
		idx: idx,
                name: modal.querySelector(`.nameInput[data-index="${idx}"]`).value.trim(),
                amount: modal.querySelector(`.amountInput[data-index="${idx}"]`).value.trim(),
                preparation_notes: modal.querySelector(`.prepInput[data-index="${idx}"]`).value.trim()
            };
        }).filter(ing => ing.name != originals[ing.idx].name || ing.amount != originals[ing.idx].amount || ing.preparation_notes != originals[ing.idx].preparation_notes).map(({idx, ...keepAttrs}) => keepAttrs);

	
	const updated_instructions = Array.from(modal.querySelectorAll('.edit-instructions'), (ing) => {
	    return {
		original_steps: ing.dataset.originalInstructions.trim(),
    		new_steps: ing.value.trim(),
	        step_name: ing.dataset.name
            }
	}).filter(ing => ing.original_steps !== ing.new_steps);

	submitRecipeChanges(recipe.id, updated_instructions, updated_ingredients, modal);
	
	// for updated ingredients if only the amount or prep notes changed we dont need a new ingredient x recipe relation
	// if name changes find ingredient or create and then change the linked keys

    });

    // Populate ingredients
    const ingredientsList = modal.querySelector('.ingredientsList');
    recipe.ingredients.forEach(ing => {
        const li = document.createElement('li');
        li.textContent = `${ing.amount} ${ing.name} ${ing.preparation_notes || ''}`.trim();
        ingredientsList.appendChild(li);
    });

    // Populatae edit ingredients
    const editIngredientsList = modal.querySelector('.editIngredientsList');
    recipe.ingredients.forEach((ing, idx) => {
    	const row = document.createElement('div');
        row.style.cssText = `
            display:flex;
            align-items:center;
            gap:10px;
            padding:6px 0;
        `;
	    row.innerHTML = `
            <input type="text" class="nameInput" data-index="${idx}" placeholder="Name" value="${ing.name || ''}">
            <input type="text" class="amountInput" data-index="${idx}" placeholder="Amount" value="${ing.amount || ''}">
            <input type="text" class="prepInput" data-index="${idx}" placeholder="Prep Notes" value="${ing.preparation_notes || ''}">
	    `;
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
        recipe.steps.main.forEach(step => {
            const li = document.createElement('li');
            li.textContent = step;
            mainOl.appendChild(li);
	        if (fullInstructions) {
	    	    fullInstructions += '\n\n';
	        }
	        fullInstructions += step
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
        title.textContent = component.charAt(0).toUpperCase() + component.slice(1);
        section.appendChild(title);

        let fullInstructions = '';
        let rows = 1;
    
        const ol = document.createElement('ol');
        steps.forEach(step => {
            const li = document.createElement('li');
            li.textContent = step;
            ol.appendChild(li);
            if (fullInstructions) {
                fullInstructions += '\n\n';
            }
            fullInstructions += step
            rows += 1;
    });
    section.appendChild(ol);
    stepsContainer.appendChild(section);
	
	const label = document.createElement('label');
	label.textContent = component.charAt(0).toUpperCase() + component.slice(1);
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

            row.innerHTML = `
                <div style="width: 200px;">${ing.name}</div>
                <input type="text" class="catInput" data-index="${ing.id}" placeholder="Category" value="${ing.category || ''}">
                <input type="text" class="locInput" data-index="${ing.id}" placeholder="Location" value="${ing.location || ''}">
                <input type="text" class="seasonInput" data-index="${ing.id}" placeholder="Season" value="${ing.season || ''}">
            `;

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
    const toggleRecipeMode = document.getElementById("toggleRecipeMode");
    const aiRecipeForm = document.getElementById("aiRecipeForm");
    const manualRecipeForm = document.getElementById("manualRecipeForm");
    const addIngredientBtn = document.getElementById("addIngredientBtn");
    addIngredientBtn.addEventListener("click", addIngredient);

    addIngredient();

    toggleRecipeMode.addEventListener("click", () => {
        const showingAI = !aiRecipeForm.hidden;

        aiRecipeForm.hidden = showingAI;
        manualRecipeForm.hidden = !showingAI;

        toggleRecipeMode.textContent = showingAI
            ? "Use AI Fill"
            : "Enter Recipe Manually";
    });
}

function toggleExpandableSection(event) {
    event?.stopPropagation();

    document
        .getElementById("expandableSection")
        ?.classList.toggle("visible");
    
    document.getElementById("displayFollowInput").value = ""
}
// contains javascript code for main application
// util
// recipe
// dark mode
// modal
//

// -------- on page load --------
window.addEventListener('DOMContentLoaded', () => {
    fetchRecipes();
    fetchIngredients();
    getSavedColorTheme();
});

// -------- global var --------
var making_grocery_list = false;
var global_ingredients = {};
var global_recipes = {};

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

function showToast(message, duration = 20000) { // 20 seconds
    const toast = document.getElementById("toast");
    toast.textContent = message;
    toast.style.opacity = "1";

    setTimeout(() => {
        toast.style.opacity = "0";
    }, duration);
}

function setMakingGroceryList() {
	making_grocery_list = !making_grocery_list
	const groceryBtn = document.getElementById("makeGroceryListBtn");
	const submitBtn = document.getElementById("submitGroceryListBtn");

	if (!making_grocery_list) {
	    groceryBtn.textContent = "Make Grocery List"
	    groceryBtn.style.backgroundColor = getComputedStyle(document.documentElement)
		.getPropertyValue('--color-accent-mint');
	   submitBtn.style.display = "none";

	    document.querySelectorAll(".recipe-card.selected")
		.forEach(card => card.classList.remove("selected"));
	} else {
	    groceryBtn.textContent = "Stop"
	    groceryBtn.style.backgroundColor = getComputedStyle(document.documentElement)
                .getPropertyValue('--color-accent-pink');
	    submitBtn.style.display = "inherit";

	    
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
		    if (!(loc in ingredient_collection)) {
		    	ingredient_collection[loc] = []
		    }
	            let ing_string = fullIngredient.name;
	            ing_string += ri.amount? `, ${ri.amount}` : '';
		    ing_string += ri.prep_notes? `, ${ri.prep_notes}` : '';
		    
	            if (fullIngredient.category === 'seasoning') {
			   	ingredient_collection['seasoning'].push(fullIngredient.name);
		    } else {
               	    	ingredient_collection[loc].push(ing_string);
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

    sortedKeys.forEach(key => {
        // Sort each array alphabetically
        ingredient_collection[key].sort((a, b) =>
            a.localeCompare(b, 'en', { sensitivity: 'base' })
        );

        output += `\`${key}\`\n`;
        output += ingredient_collection[key].join("\n") + "\n\n";
    });

    console.log(output.trim());
}

// -------- recipe code --------
async function fetchRecipes() {
    const btn = document.getElementById('recipesBtn');
    const responseDiv = document.getElementById('recipesResponse');

    btn.disabled = true;
    responseDiv.className = 'loading';
    responseDiv.textContent = 'Loading recipes...';
    try {
        const response = await fetch('/recipes', {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' }
        });

        if (!response.ok) throw new Error('HTTP ' + response.status);

        const recipes = await response.json();
	
        if (recipes.length) {
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
        // Example API call — update this for your backend
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
}

function closeModal(modalElement) {
    const modal = typeof modalElement === 'string'
        ? document.getElementById(modalElement)
        : modalElement;

    modal.classList.remove('visible');
    setTimeout(() => {
        modal.style.display = 'none';
    }, 200);
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
        <div class="modal-content" style="max-width: 800px; width: 70%;">
            <span class="close">&times;</span>
            <h2>${recipe.title}</h2>
            <h3>Ingredients</h3>
            <ul class="ingredientsList"></ul>
            <div class="stepsContainer"></div>
        </div>
    `;

    document.body.appendChild(modal);

    modal.querySelector('.close').addEventListener('click', () => closeModal(modal));
    card.addEventListener('click', () => {
	if (making_grocery_list) {
		card.classList.toggle("selected");
	} else {
        	openModal(modal);
	}
    });

    // Populate ingredients
    const ingredientsList = modal.querySelector('.ingredientsList');
    recipe.ingredients.forEach(ing => {
        const li = document.createElement('li');
        li.textContent = `${ing.amount} ${ing.name} ${ing.preparation_notes || ''}`.trim();
        ingredientsList.appendChild(li);
    });

    // Populate steps
    const stepsContainer = modal.querySelector('.stepsContainer');
    if (recipe.steps.main) {
        const mainSection = document.createElement('div');
        const mainTitle = document.createElement('h3');
        mainTitle.textContent = 'Instructions';
        mainSection.appendChild(mainTitle);
    
        const mainOl = document.createElement('ol');
        recipe.steps.main.forEach(step => {
            const li = document.createElement('li');
            li.textContent = step;
            mainOl.appendChild(li);
        });
        mainSection.appendChild(mainOl);
        stepsContainer.appendChild(mainSection);
    }
    
    // Add other subcomponents
    Object.entries(recipe.steps).forEach(([component, steps]) => {
        if (component === 'main') return;
    
        const section = document.createElement('div');
        const title = document.createElement('h3');
        title.textContent = component.charAt(0).toUpperCase() + component.slice(1);
        section.appendChild(title);
    
        const ol = document.createElement('ol');
        steps.forEach(step => {
            const li = document.createElement('li');
            li.textContent = step;
            ol.appendChild(li);
        });
        section.appendChild(ol);
        stepsContainer.appendChild(section);
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
            <h2>Tag Ingredients</h2>

            <h3>Ingredients</h3>
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

    ingredients.forEach((ing, idx) => {
        const row = document.createElement("div");
        row.style.cssText = `
            display:flex;
            align-items:center;
            gap:10px;
            padding:6px 0;
        `;

        row.innerHTML = `
            <div style="width: 200px;">${ing.name}</div>
            <input type="text" class="catInput" data-index="${idx}" placeholder="Category" value="${ing.category || ''}">
            <input type="text" class="locInput" data-index="${idx}" placeholder="Location" value="${ing.location || ''}">
	    <input type="text" class="seasonInput" data-index="${idx}" placeholder="Season" value="${ing.season || ''}">
        `;

        listContainer.appendChild(row);
    });

    // Save handler
    modal.querySelector("#saveIngredientTagsBtn").addEventListener("click", async () => {
        const updated = ingredients.map((ing, idx) => {
            return {
                ...ing,
                category: modal.querySelector(`.catInput[data-index="${idx}"]`).value.trim(),
                location: modal.querySelector(`.locInput[data-index="${idx}"]`).value.trim(),
		season: modal.querySelector(`.seasonInput[data-index="${idx}"]`).value.trim()
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

    	} catch (err) {
        	console.error("Failed to save ingredients:", err);
        	showToast("Error saving ingredients.");
    	}
    });

}


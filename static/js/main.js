// contains javascript code for main application
// util
// recipe
// dark mode
// modal
//

// -------- on page load --------
window.addEventListener('DOMContentLoaded', () => {
    fetchRecipes();

    getSavedColorTheme();
});

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

        // Reload recipes
        fetchRecipes();

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
    // Create the modal dynamically
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
            <ul id="ingredientsList"></ul>
            <div id="stepsContainer"></div>
        </div>
    `;

    document.body.appendChild(modal);

    // Close button
    modal.querySelector('.close').addEventListener('click', () => closeModal(modal));

    // Populate ingredients
    const ingredientsList = modal.querySelector('#ingredientsList');
    recipe.ingredients.forEach(ing => {
        const li = document.createElement('li');
        li.textContent = `${ing.amount} ${ing.name} ${ing.preparation_notes || ''}`.trim();
        ingredientsList.appendChild(li);
    });

    // Populate steps
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
    card.addEventListener('click', () => {
        modal.style.display = 'block';
    });
}

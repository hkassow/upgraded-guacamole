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
        responseDiv.className = 'error';
        responseDiv.textContent = 'Error:\n' + error.message;
    } finally {
        btn.disabled = false;
    }
}

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
                    <h3>${r.name}</h3>
                `;
                card.addEventListener('click', () => {
                    console.log('Clicked recipe:', r);
                });
                responseDiv.appendChild(card);
            });
        }
    } catch (error) {
        responseDiv.className = 'error';
        responseDiv.textContent = 'Error:\n' + error.message;
    } finally {
        btn.disabled = false;
    }
}
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
    });
}
// Automatically fetch recipes when the page loads
window.addEventListener('DOMContentLoaded', () => {
    fetchRecipes();
    
    getSavedColorTheme();
});


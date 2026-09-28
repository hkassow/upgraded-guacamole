// Recipe list filtering: a search box (titles, ingredients and tags), whose recipes (mine /
// following) and tag chips.
//
// fetchRecipes() in main.js draws the recipe cards and then calls renderRecipeFilters(recipes).
// Filtering only hides cards, so recipes selected for a grocery list stay selected while you
// change filters.

const recipeFilterState = {
    search: '',
    owner: 'all', // 'all' | 'mine' | 'following'
    tags: new Set(),
    showAllTags: false,
};

// the most used tags are shown; the rest sit behind a "+N more" button
const VISIBLE_TAG_CHIPS = 8;

let filterableRecipes = [];

function renderRecipeFilters(recipes) {
    filterableRecipes = recipes;
    const container = document.getElementById('recipeFilters');
    container.innerHTML = '';

    if (!recipes.length) {
        container.hidden = true;
        applyRecipeFilters();
        return;
    }
    container.hidden = false;

    container.appendChild(createFilterSearch());

    // only worth showing when the list actually mixes your recipes with people you follow
    const hasMine = recipes.some(r => r.is_mine);
    const hasOthers = recipes.some(r => !r.is_mine);
    if (hasMine && hasOthers) {
        container.appendChild(createOwnerFilter());
    } else {
        recipeFilterState.owner = 'all';
    }

    const tagFilter = createTagFilter(recipes);
    if (tagFilter) container.appendChild(tagFilter);

    container.appendChild(createFilterStatus());
    applyRecipeFilters();
}

function createFilterSearch() {
    const search = document.createElement('input');
    search.type = 'search';
    search.className = 'filter-search';
    search.placeholder = 'Search recipes or ingredients';
    search.setAttribute('aria-label', 'Search recipes or ingredients');
    search.value = recipeFilterState.search;
    search.addEventListener('input', () => {
        recipeFilterState.search = search.value;
        applyRecipeFilters();
    });
    return search;
}

// a single segmented switch, so it reads differently from the tag chips
function createOwnerFilter() {
    const group = document.createElement('div');
    group.className = 'filter-segmented';
    group.setAttribute('role', 'group');
    group.setAttribute('aria-label', 'Whose recipes');

    [['all', 'All'], ['mine', 'Mine'], ['following', 'Following']].forEach(([value, label]) => {
        const chip = createFilterChip(label, recipeFilterState.owner === value);
        chip.className = 'filter-segment';
        chip.addEventListener('click', () => {
            recipeFilterState.owner = value;
            group.querySelectorAll('.filter-segment').forEach(c => c.setAttribute('aria-pressed', String(c === chip)));
            applyRecipeFilters();
        });
        group.appendChild(chip);
    });
    return group;
}

// chips for every tag on the listed recipes, most used first
function createTagFilter(recipes) {
    const counts = new Map();
    recipes.forEach(r => (r.tags || []).forEach(tag => counts.set(tag, (counts.get(tag) || 0) + 1)));

    // forget selected tags that no longer exist (e.g. removed in an edit)
    [...recipeFilterState.tags].forEach(tag => {
        if (!counts.has(tag)) recipeFilterState.tags.delete(tag);
    });
    if (!counts.size) return null;

    const group = document.createElement('div');
    group.className = 'filter-chips';
    group.setAttribute('role', 'group');
    group.setAttribute('aria-label', 'Filter by tag');

    const sorted = [...counts.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
    const overflow = [];

    sorted.forEach(([tag, count], index) => {
        const chip = createFilterChip(tag, recipeFilterState.tags.has(tag));
        chip.title = `${count} recipe${count === 1 ? '' : 's'}`;
        chip.addEventListener('click', () => {
            if (recipeFilterState.tags.has(tag)) {
                recipeFilterState.tags.delete(tag);
            } else {
                recipeFilterState.tags.add(tag);
            }
            chip.setAttribute('aria-pressed', String(recipeFilterState.tags.has(tag)));
            applyRecipeFilters();
        });

        // selected tags always stay visible, even past the cut-off
        if (index >= VISIBLE_TAG_CHIPS && !recipeFilterState.tags.has(tag)) {
            chip.hidden = !recipeFilterState.showAllTags;
            overflow.push(chip);
        }
        group.appendChild(chip);
    });

    if (overflow.length) {
        const more = document.createElement('button');
        more.type = 'button';
        more.className = 'filter-clear filter-more';
        const label = () => recipeFilterState.showAllTags ? 'Show fewer' : `+${overflow.length} more`;
        more.textContent = label();
        more.setAttribute('aria-expanded', String(recipeFilterState.showAllTags));
        more.addEventListener('click', () => {
            recipeFilterState.showAllTags = !recipeFilterState.showAllTags;
            overflow.forEach(chip => { chip.hidden = !recipeFilterState.showAllTags; });
            more.textContent = label();
            more.setAttribute('aria-expanded', String(recipeFilterState.showAllTags));
        });
        group.appendChild(more);
    }
    return group;
}

function createFilterChip(label, pressed) {
    const chip = document.createElement('button');
    chip.type = 'button';
    chip.className = 'filter-chip';
    chip.textContent = label;
    chip.setAttribute('aria-pressed', String(pressed));
    return chip;
}

function createFilterStatus() {
    const status = document.createElement('div');
    status.className = 'filter-status';

    const count = document.createElement('span');
    count.id = 'recipeFilterCount';
    count.setAttribute('aria-live', 'polite');

    const clear = document.createElement('button');
    clear.type = 'button';
    clear.id = 'recipeFilterClear';
    clear.className = 'filter-clear';
    clear.textContent = 'Clear filters';
    clear.addEventListener('click', () => {
        recipeFilterState.search = '';
        recipeFilterState.owner = 'all';
        recipeFilterState.tags.clear();
        renderRecipeFilters(filterableRecipes);
    });

    status.append(count, clear);
    return status;
}

function recipeFiltersActive() {
    return recipeFilterState.search.trim() !== '' || recipeFilterState.owner !== 'all' || recipeFilterState.tags.size > 0;
}

function recipeMatchesFilters(recipe) {
    if (recipeFilterState.owner === 'mine' && !recipe.is_mine) return false;
    if (recipeFilterState.owner === 'following' && recipe.is_mine) return false;

    // every selected tag must be on the recipe ("vegan" + "dinner" narrows down)
    const tags = recipe.tags || [];
    for (const tag of recipeFilterState.tags) {
        if (!tags.includes(tag)) return false;
    }

    // every search word must appear in the title, an ingredient or a tag ("halloumi salad")
    const words = recipeFilterState.search.toLowerCase().split(/\s+/).filter(Boolean);
    if (words.length) {
        const haystack = [
            recipe.title,
            ...(recipe.ingredients || []).map(ing => ing.name),
            ...tags,
        ].join(' ').toLowerCase();
        if (!words.every(word => haystack.includes(word))) return false;
    }
    return true;
}

function applyRecipeFilters() {
    const cards = document.querySelectorAll('#recipesResponse .recipe-card');
    let shown = 0;
    cards.forEach(card => {
        const recipe = global_recipes[card.dataset.id];
        const visible = !recipe || recipeMatchesFilters(recipe);
        card.hidden = !visible;
        if (visible) shown++;
    });

    const active = recipeFiltersActive();
    const count = document.getElementById('recipeFilterCount');
    if (count) {
        const noun = cards.length === 1 ? 'recipe' : 'recipes';
        count.textContent = active ? `${shown} of ${cards.length} ${noun}` : `${cards.length} ${noun}`;
    }
    const clear = document.getElementById('recipeFilterClear');
    if (clear) clear.hidden = !active;

    document.getElementById('recipeFilterEmpty').hidden = !(cards.length > 0 && shown === 0);
}

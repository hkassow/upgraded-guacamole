// Recipe list filtering: a search box (titles, ingredients and tags), whose recipes (mine /
// following) and tags.
//
// Layout: the search box and a "Filters" button are always shown; the button opens a panel with
// the Mine/Following switch and tag chips. With lots of tags only the most used get a chip, and
// "All tags" opens a dialog with every tag and its own search box.
//
// fetchRecipes() in main.js draws the recipe cards and then calls renderRecipeFilters(recipes).
// Filtering only hides cards, so recipes selected for a grocery list stay selected while you
// change filters.

const recipeFilterState = {
    search: '',
    owner: 'all', // 'all' | 'mine' | 'following'
    tags: new Set(),
};

// up to this many tags are all shown as chips...
const SHOW_ALL_TAG_CHIPS_UP_TO = 10;
// ...past that, only the most used get a chip (plus any selected ones) and the rest are in "All tags"
const TOP_TAG_CHIPS = 6;

// whether the filter panel is open is remembered per browser
const FILTERS_OPEN_KEY = 'recipeFiltersOpen';

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

    const panel = document.createElement('div');
    panel.id = 'recipeFilterPanel';
    panel.className = 'filter-panel';

    // only worth showing when the list actually mixes your recipes with people you follow
    const hasMine = recipes.some(r => r.is_mine);
    const hasOthers = recipes.some(r => !r.is_mine);
    if (hasMine && hasOthers) {
        panel.appendChild(createOwnerFilter());
    } else {
        recipeFilterState.owner = 'all';
    }

    const tagFilter = createTagFilter(countRecipeTags(recipes));
    if (tagFilter) panel.appendChild(tagFilter);

    const top = document.createElement('div');
    top.className = 'filter-top';
    top.appendChild(createFilterSearch());

    // no switch and no tags means nothing to put behind the button
    if (panel.children.length) {
        const open = filtersPanelOpen();
        panel.hidden = !open;
        top.appendChild(createFiltersButton(panel, open));
        container.append(top, panel);
    } else {
        container.append(top);
    }

    container.appendChild(createFilterStatus());
    applyRecipeFilters();
}

function filtersPanelOpen() {
    try {
        return localStorage.getItem(FILTERS_OPEN_KEY) === 'true';
    } catch (e) {
        return false; // storage blocked: start collapsed
    }
}

function rememberFiltersPanelOpen(open) {
    try {
        localStorage.setItem(FILTERS_OPEN_KEY, String(open));
    } catch (e) {
        // not remembered, that's fine
    }
}

function createFilterSearch() {
    const search = document.createElement('input');
    search.type = 'search';
    search.className = 'filter-search';
    search.placeholder = 'Search recipes'; // short so it fits beside the Filters button on phones
    search.setAttribute('aria-label', 'Search recipes or ingredients');
    search.value = recipeFilterState.search;
    search.addEventListener('input', () => {
        recipeFilterState.search = search.value;
        applyRecipeFilters();
    });
    return search;
}

// shows/hides the panel; its label says how many filters are active so collapsed filters aren't forgotten
function createFiltersButton(panel, open) {
    const button = document.createElement('button');
    button.type = 'button';
    button.id = 'recipeFiltersButton';
    button.className = 'filters-button';
    button.setAttribute('aria-controls', panel.id);
    button.setAttribute('aria-expanded', String(open));
    button.addEventListener('click', () => {
        const open = panel.hidden;
        panel.hidden = !open;
        button.setAttribute('aria-expanded', String(open));
        rememberFiltersPanelOpen(open);
    });
    return button;
}

// number of filters hidden behind the Filters button (search is always visible, so not counted)
function panelFilterCount() {
    return recipeFilterState.tags.size + (recipeFilterState.owner !== 'all' ? 1 : 0);
}

// a single segmented switch, so it reads differently from the tag chips
function createOwnerFilter() {
    const group = document.createElement('div');
    group.className = 'filter-segmented';
    group.setAttribute('role', 'group');
    group.setAttribute('aria-label', 'Whose recipes');

    [['all', 'All'], ['mine', 'Mine'], ['following', 'Following']].forEach(([value, label]) => {
        const segment = createFilterChip(label, recipeFilterState.owner === value);
        segment.className = 'filter-segment';
        segment.addEventListener('click', () => {
            recipeFilterState.owner = value;
            group.querySelectorAll('.filter-segment').forEach(s => s.setAttribute('aria-pressed', String(s === segment)));
            applyRecipeFilters();
        });
        group.appendChild(segment);
    });
    return group;
}

// [tag, number of recipes] pairs, most used first
function countRecipeTags(recipes) {
    const counts = new Map();
    recipes.forEach(r => (r.tags || []).forEach(tag => counts.set(tag, (counts.get(tag) || 0) + 1)));

    // forget selected tags that no longer exist (e.g. removed in an edit)
    [...recipeFilterState.tags].forEach(tag => {
        if (!counts.has(tag)) recipeFilterState.tags.delete(tag);
    });

    return [...counts.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
}

function toggleTagFilter(tag) {
    if (recipeFilterState.tags.has(tag)) {
        recipeFilterState.tags.delete(tag);
    } else {
        recipeFilterState.tags.add(tag);
    }
    applyRecipeFilters();
}

function createTagFilter(tagCounts) {
    if (!tagCounts.length) return null;

    const group = document.createElement('div');
    group.className = 'filter-chips';
    group.setAttribute('role', 'group');
    group.setAttribute('aria-label', 'Filter by tag');

    const collapsed = tagCounts.length > SHOW_ALL_TAG_CHIPS_UP_TO;
    const chipTags = collapsed
        ? tagCounts.filter(([tag], index) => index < TOP_TAG_CHIPS || recipeFilterState.tags.has(tag))
        : tagCounts;

    chipTags.forEach(([tag, count]) => {
        const chip = createFilterChip(tag, recipeFilterState.tags.has(tag));
        chip.title = `${count} recipe${count === 1 ? '' : 's'}`;
        chip.addEventListener('click', () => {
            toggleTagFilter(tag);
            chip.setAttribute('aria-pressed', String(recipeFilterState.tags.has(tag)));
        });
        group.appendChild(chip);
    });

    if (collapsed) {
        const allTags = document.createElement('button');
        allTags.type = 'button';
        allTags.className = 'filter-chip filter-all-tags';
        allTags.textContent = `All tags (${tagCounts.length})`;
        allTags.setAttribute('aria-haspopup', 'dialog');
        allTags.addEventListener('click', () => openAllTagsDialog(tagCounts));
        group.appendChild(allTags);
    }
    return group;
}

// every tag with a checkbox and count, plus a box to find one; changes apply straight away
function openAllTagsDialog(tagCounts) {
    const dialog = document.createElement('dialog');
    dialog.className = 'tag-dialog';
    dialog.setAttribute('aria-labelledby', 'tagDialogTitle');

    const header = document.createElement('div');
    header.className = 'tag-dialog-header';
    const title = document.createElement('h2');
    title.id = 'tagDialogTitle';
    title.textContent = 'All tags';
    // start focus on the heading rather than the close button (or the search box, which would pop
    // up the keyboard on phones)
    title.tabIndex = -1;
    title.autofocus = true;
    const close = document.createElement('button');
    close.type = 'button';
    close.className = 'tag-dialog-close';
    close.textContent = '×';
    close.setAttribute('aria-label', 'Close');
    close.addEventListener('click', () => dialog.close());
    header.append(title, close);

    const find = document.createElement('input');
    find.type = 'search';
    find.className = 'filter-search';
    find.placeholder = 'Find a tag';
    find.setAttribute('aria-label', 'Find a tag');

    const list = document.createElement('ul');
    list.className = 'tag-dialog-list';
    tagCounts.forEach(([tag, count]) => {
        const item = document.createElement('li');
        item.dataset.tag = tag;
        const label = document.createElement('label');
        const checkbox = document.createElement('input');
        checkbox.type = 'checkbox';
        checkbox.checked = recipeFilterState.tags.has(tag);
        checkbox.addEventListener('change', () => toggleTagFilter(tag));
        const name = document.createElement('span');
        name.textContent = tag;
        const countText = document.createElement('span');
        countText.className = 'tag-dialog-count';
        countText.textContent = count;
        countText.setAttribute('aria-label', `${count} recipe${count === 1 ? '' : 's'}`);
        label.append(checkbox, name, countText);
        item.appendChild(label);
        list.appendChild(item);
    });

    const noMatch = document.createElement('p');
    noMatch.className = 'filter-empty';
    noMatch.textContent = 'No tags match';
    noMatch.hidden = true;

    find.addEventListener('input', () => {
        const query = find.value.trim().toLowerCase();
        let shown = 0;
        list.querySelectorAll('li').forEach(item => {
            item.hidden = !item.dataset.tag.includes(query);
            if (!item.hidden) shown++;
        });
        noMatch.hidden = shown > 0;
    });

    const done = document.createElement('button');
    done.type = 'button';
    done.className = 'tag-dialog-done';
    done.textContent = 'Done';
    done.addEventListener('click', () => dialog.close());

    dialog.append(header, find, list, noMatch, done);

    // clicking the dimmed area outside the dialog closes it (closeOnBackdropClick is in main.js)
    closeOnBackdropClick(dialog, () => dialog.close());
    // redraw so tags picked here show up as chips, then put focus back where it was
    dialog.addEventListener('close', () => {
        dialog.remove();
        renderRecipeFilters(filterableRecipes);
        document.querySelector('.filter-all-tags')?.focus();
    });

    document.body.appendChild(dialog);
    dialog.showModal();
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
    return recipeFilterState.search.trim() !== '' || panelFilterCount() > 0;
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

    const button = document.getElementById('recipeFiltersButton');
    if (button) {
        const hidden = panelFilterCount();
        button.textContent = hidden ? `Filters (${hidden})` : 'Filters';
        button.classList.toggle('has-active', hidden > 0);
    }

    document.getElementById('recipeFilterEmpty').hidden = !(cards.length > 0 && shown === 0);
}

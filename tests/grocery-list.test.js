// Frontend unit tests: node --test tests/
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

// grocery-list.js is a plain browser script, so run it the way the page does: with conversions.js
// loaded first as a global
const context = vm.createContext({ IngredientConversions: require('../static/js/conversions.js') });
vm.runInContext(fs.readFileSync(path.join(__dirname, '../static/js/grocery-list.js'), 'utf8'), context);
const { buildShoppingList, buildIngredientCollection, parseShoppingAmount } = context;

const linesFor = (list, name) => list.find(item => item.name === name)?.lines;

test('reads grams written as "gr" and comma decimals', () => {
    assert.deepEqual({ ...parseShoppingAmount('150 gr') }, { weight: 150 });
    assert.deepEqual({ ...parseShoppingAmount('7,5 gr') }, { weight: 7.5 });
    assert.deepEqual({ ...parseShoppingAmount('3') }, { count: 3 });
    assert.equal(Math.round(parseShoppingAmount('packed ¼ cup').volume), 59);
});

test('never adds an unreadable amount up as a count', () => {
    for (const amount of ['2-3 cups', '1 can', '2 large', 'a pinch', '1 (14 oz) can', '']) {
        assert.equal(parseShoppingAmount(amount), null, amount);
    }
});

// amounts from a real grocery list where "150 gr" was being counted as 150 items
test('combines the brioche and babka amounts correctly', () => {
    const list = buildShoppingList([
        // brioche
        { name: 'flour', amount: '500 gr' },
        { name: 'sugar', amount: '70 gr' },
        { name: 'yeast', amount: '7,5 gr' },
        { name: 'eggs', amount: '150 gr' },
        { name: 'milk', amount: '150 gr' },
        { name: 'butter', amount: '150 gr' },
        // babka and others
        { name: 'flour', amount: '4¼ cups' },
        { name: 'sugar', amount: '¼ cup' },
        { name: 'sugar', amount: 'packed ¼ cup' },
        { name: 'yeast', amount: '2 teaspoons' },
        { name: 'eggs', amount: '3' },
        { name: 'milk', amount: '¾ cup plus 2 tablespoons' },
        { name: 'butter', amount: '½ cup plus 2 tablespoons' },
        { name: 'butter', amount: '' },
    ]);

    // everything for a known ingredient ends up as one total
    assert.deepEqual([...linesFor(list, 'flour')], ['≈ 1.01 kg']);  // 500 g + 4¼ cups (510 g)
    assert.deepEqual([...linesFor(list, 'sugar')], ['≈ 170 g']);    // 70 g + ¼ cup + packed ¼ cup
    assert.deepEqual([...linesFor(list, 'butter')], ['≈ 290 g']);   // 150 g + ½ cup plus 2 tbsp (142 g)
    assert.deepEqual([...linesFor(list, 'milk')], ['≈ 350 ml']);    // 150 g (146 ml) + ¾ cup plus 2 tbsp (207 ml)

    // not in the density table: grams and counts stay separate instead of being added together
    assert.deepEqual([...linesFor(list, 'eggs')], ['150g', '3']);
    assert.deepEqual([...linesFor(list, 'yeast')], ['8g', '2 tsp']);
});

test('lists every original amount per ingredient for sanity checking', () => {
    const ingredientsById = {
        1: { name: 'butter', location: 'dairy', category: 'dairy' },
        2: { name: 'Butter', location: 'dairy', category: 'dairy' }, // same ingredient, different case
        3: { name: 'salt', location: 'pantry', category: 'seasoning' },
        4: { name: 'orange zest', location: 'produce' },
    };
    const recipesById = {
        10: { title: 'Brioche', ingredients: [
            { ingredient_id: 1, amount: '150 gr' },
            { ingredient_id: 3, amount: '9 gr' },
        ] },
        11: { title: 'Babka', ingredients: [
            { ingredient_id: 2, amount: '½ cup plus 2 tablespoons' },
            { ingredient_id: 1, amount: '' }, // "extra softened, for greasing"
            { ingredient_id: 3, amount: '1 teaspoon' },
            { ingredient_id: 4, amount: '' },
        ] },
    };

    const collection = buildIngredientCollection([10, 11], recipesById, ingredientsById);

    assert.deepEqual(JSON.parse(JSON.stringify(collection.originalAmounts)), [
        { name: 'butter', amounts: ['150 gr', '½ cup plus 2 tablespoons'], total: '≈ 290 g' },
        { name: 'orange zest', amounts: [], total: '' },
        { name: 'salt', amounts: ['9 gr', '1 teaspoon'], total: '9g + 1 tsp' },
    ]);
});

test('formats volumes without trailing zeros', () => {
    const list = buildShoppingList([
        { name: 'adzuki beans', amount: '⅔ cup' },
        { name: 'baking powder', amount: '2 teaspoons' },
        { name: 'nori flakes', amount: '3 tablespoons' },
        { name: 'hazelnuts', amount: '6' },
    ]);
    assert.deepEqual([...linesFor(list, 'adzuki beans')], ['⅔ cup']);   // was "10.67 tbsp"
    assert.deepEqual([...linesFor(list, 'baking powder')], ['2 tsp']); // was "2.00 tsp"
    assert.deepEqual([...linesFor(list, 'nori flakes')], ['3 tbsp']);
    assert.deepEqual([...linesFor(list, 'hazelnuts')], ['6']);
});

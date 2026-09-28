// Frontend unit tests: node --test tests/
const test = require('node:test');
const assert = require('node:assert/strict');

const conversions = require('../static/js/conversions.js');

test('parses amounts the parser produces', () => {
    const ml = (amount) => Math.round(conversions.parseAmount(amount).base);

    assert.equal(ml('1 cup'), 237);
    assert.equal(ml('½ cup'), 118);
    assert.equal(ml('2¾ cups'), 651);
    assert.equal(ml('1 ½ cups'), 355);
    assert.equal(ml('1 1/2 cups'), 355);
    assert.equal(ml('1/2 cup'), 118);
    assert.equal(ml('0.5 cup'), 118);
    assert.equal(ml('3 tbsp'), 44);
    assert.equal(ml('2 tablespoons'), 30);
    assert.equal(ml('1 teaspoon'), 5);
    assert.equal(ml('480 ml'), 480);
    assert.equal(ml('480ml'), 480);
    assert.equal(ml('1.5 l'), 1500);
    assert.equal(ml('½ cup plus 2 tablespoons'), 148);
    assert.equal(ml('packed ⅔ cup'), 158);
    assert.equal(ml('scant ¾ cup'), 177);

    assert.deepEqual(conversions.parseAmount('250g'), { family: 'weight', base: 250, metric: true });
    assert.equal(Math.round(conversions.parseAmount('10 ounces').base), 283);
    assert.equal(Math.round(conversions.parseAmount('1 lb').base), 454);
    assert.equal(conversions.parseAmount('1.2 kg').base, 1200);
});

test('does not parse things that are not measurements', () => {
    for (const amount of ['3', '3 large', '1 can', '1 (14 oz) can', '2-3 cups', '2 to 3 cups', 'pinch', '', 'remaining', '1 cup plus 50 g', '2 cloves']) {
        assert.equal(conversions.parseAmount(amount), null, amount);
    }
});

test('density table is well formed', () => {
    const seen = new Map();
    for (const entry of conversions.table) {
        assert.ok(entry.gramsPerCup > 0, entry.names[0]);
        assert.ok(['weight', 'volume'].includes(entry.measure), entry.names[0]);
        for (const name of entry.names) {
            // names are written lowercase with no dashes/accents/apostrophes, i.e. already normalized
            assert.equal(conversions.normalizeName(name), name, `"${name}" isn't normalized`);
            // a name listed twice would silently use whichever entry came last
            assert.ok(!seen.has(name), `"${name}" is in both "${seen.get(name)}" and "${entry.names[0]}"`);
            seen.set(name, entry.names[0]);
        }
    }
});

test('looks up known ingredients by exact name', () => {
    for (const name of ['granulated sugar', 'Sugar', 'dark brown muscovado sugar', 'all-purpose flour', 'All-Purpose Flour',
        'unsalted butter', 'softened butter', 'whole milk', 'heavy cream', 'full-fat greek yogurt', 'lightly packed brown sugar',
        'whiskey or rum', "confectioners' sugar", 'Crème fraîche', 'half & half', 'self-rising flour', 'frozen blueberries',
        'large ripe mashed bananas', 'coconut sugar', 'rice flour']) {
        assert.ok(conversions.lookup(name), name);
    }
    // anything not in the table is left alone - no guessing from a word inside the name
    for (const name of ['cream of tartar', 'ice cream', 'eggs', 'large eggs', 'salt', 'kosher salt', 'baking soda',
        'dark chocolate', 'butternut squash', 'vanilla extract', 'xanthan gum', 'nutritional yeast', 'corn tortillas', '']) {
        assert.equal(conversions.lookup(name), null, name);
    }
});

test('uses the King Arthur chart values', () => {
    const grams = (name, amount = '1 cup') => conversions.alternativeAmount(name, amount);
    assert.equal(grams('cake flour'), '≈ 120 g');
    assert.equal(grams('caster sugar'), '≈ 190 g');
    assert.equal(grams('almond meal'), '≈ 85 g');       // 84
    assert.equal(grams('almond flour'), '≈ 95 g');      // 96
    assert.equal(grams('cornmeal'), '≈ 155 g');         // 156, regular yellow cornmeal
    assert.equal(grams('whole grain cornmeal'), '≈ 140 g'); // 138
    assert.equal(grams('cornstarch', '¼ cup'), '≈ 30 g');   // 28
    assert.equal(grams('tahini', '½ cup'), '≈ 130 g');      // 128
    assert.equal(grams('unsweetened shredded coconut'), '≈ 55 g'); // 53, vs 85 for sweetened
    assert.equal(grams('shredded coconut'), '≈ 85 g');
});

test('shows grams for dry ingredients measured in cups', () => {
    assert.equal(conversions.alternativeAmount('granulated sugar', '2½ cups'), '≈ 495 g');
    assert.equal(conversions.alternativeAmount('all-purpose flour', '5¼ cups'), '≈ 630 g');
    assert.equal(conversions.alternativeAmount('honey', 'scant ¾ cup'), '≈ 250 g');
    assert.equal(conversions.alternativeAmount('unsalted butter', '¾ cup'), '≈ 170 g');
    assert.equal(conversions.alternativeAmount('granulated sugar', '½ cup plus 2 tablespoons'), '≈ 125 g');
    assert.equal(conversions.alternativeAmount('sugar', '10 cups'), '≈ 1.98 kg');
});

test('shows cups for dry ingredients given in grams', () => {
    assert.equal(conversions.alternativeAmount('flour', '500 g'), '≈ 4¼ cups'); // 4.17 cups
    assert.equal(conversions.alternativeAmount('granulated sugar', '100g'), '≈ ½ cup');
    assert.equal(conversions.alternativeAmount('butter', '8 oz'), '≈ 1 cup');
    assert.equal(conversions.alternativeAmount('cocoa powder', '30 g'), '≈ ⅓ cup'); // 0.36 cups
});

test('converts liquids between cups and ml', () => {
    assert.equal(conversions.alternativeAmount('heavy cream', '2 cups'), '≈ 475 ml');
    assert.equal(conversions.alternativeAmount('whole milk', '480 ml'), '≈ 2 cups');
    assert.equal(conversions.alternativeAmount('whiskey or rum', '½ cup'), '≈ 120 ml');
    assert.equal(conversions.alternativeAmount('milk', '1 quart'), '≈ 945 ml');
    assert.equal(conversions.alternativeAmount('water', '1.5 l'), '≈ 6⅓ cups');
});

test('skips small amounts and unknown ingredients', () => {
    assert.equal(conversions.alternativeAmount('granulated sugar', '1 tbsp'), null);
    assert.equal(conversions.alternativeAmount('butter', '1 teaspoon'), null);
    assert.equal(conversions.alternativeAmount('flour', '10 g'), null);
    assert.equal(conversions.alternativeAmount('salt', '1 cup'), null);
    assert.equal(conversions.alternativeAmount('eggs', '3'), null);
    assert.equal(conversions.alternativeAmount('granulated sugar', '2-3 cups'), null);
});

test('formats cups to the nearest quarter or third', () => {
    const cup = 236.588;
    assert.equal(conversions.formatCups(cup), '1 cup');
    assert.equal(conversions.formatCups(cup * 0.26), '¼ cup');
    assert.equal(conversions.formatCups(cup * 0.34), '⅓ cup');
    assert.equal(conversions.formatCups(cup * 0.97), '1 cup');
    assert.equal(conversions.formatCups(cup * 1.68), '1⅔ cups');
    assert.equal(conversions.formatCups(cup * 2.9), '3 cups');
    assert.equal(conversions.formatCups(cup * 0.1), '2 tbsp');
});

test('combines grocery totals into grams or ml', () => {
    // 1 cup + 100 g sugar -> one weight total
    let result = conversions.combineTotals('granulated sugar', { volume: 236.588, weight: 100 });
    assert.equal(result.converted, true);
    assert.deepEqual(Object.keys(result.totals), ['weight']);
    assert.equal(Math.round(result.totals.weight), 298);

    // 500 g milk + 1 cup milk -> one volume total
    result = conversions.combineTotals('milk', { volume: 236.588, weight: 500 });
    assert.deepEqual(Object.keys(result.totals), ['volume']);
    assert.equal(Math.round(result.totals.volume), 721);

    // counts and already-right units are kept as they are
    result = conversions.combineTotals('butter', { weight: 200, count: 2 });
    assert.equal(result.converted, false);
    assert.deepEqual(result.totals, { weight: 200, count: 2 });

    assert.equal(conversions.combineTotals('eggs', { count: 3 }), null);
});

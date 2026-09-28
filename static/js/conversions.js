// Weight <-> volume conversions for common ingredients, e.g. "2½ cups sugar" -> "≈ 495 g" or
// "500 g flour" -> "≈ 4 cups".
//
// Only ingredients in INGREDIENT_DENSITIES are converted, matched by exact name (ignoring a few
// harmless words like "unsalted" or "softened") - anything else returns null. Dry and solid things
// (flour, sugar, butter, honey...) convert to/from grams; liquids (milk, cream, oil...) to/from ml.
// Results are approximate, so they're shown with "≈".
//
// Used by the recipe view (when an ingredient has no alt_amount) and the grocery list. Also loads
// in node for tests: node --test tests/

const IngredientConversions = (() => {
    const ML_PER_CUP = 236.588;

    // Grams per US cup.
    //
    // Dry and solid ingredients: King Arthur Baking's ingredient weight chart, scaled to one cup
    // (e.g. "cornstarch ¼ cup = 28 g" -> 112). Where the chart gives a range the middle is used.
    //
    // Liquids use their physical density instead. The chart lists water, milk and cream at 227 g
    // ("8 oz per cup") and oil at ~200 g, but a cup of water weighs 236.6 g by definition. Liquids
    // are converted cups <-> ml, so their density only matters for grams <-> ml.
    //
    // Names are lowercase with no dashes, accents or apostrophes (the lookup strips them from
    // ingredient names too). Add aliases to "names" as new ingredient names show up.
    //
    // Deliberately left out: salt (Diamond kosher 128 g/cup vs table salt 288 - a recipe's "salt" is
    // too ambiguous), eggs (counted, not measured) and things only used by the teaspoon.
    const INGREDIENT_DENSITIES = [
        // wheat flours
        { names: ['flour', 'all purpose flour', 'plain flour', 'white flour', 'unbleached flour'], gramsPerCup: 120, measure: 'weight' },
        { names: ['bread flour', 'strong flour', 'strong white flour', 'high gluten flour'], gramsPerCup: 120, measure: 'weight' },
        { names: ['00 flour', 'tipo 00 flour', '00 pizza flour', 'pizza flour'], gramsPerCup: 116, measure: 'weight' },
        { names: ['cake flour'], gramsPerCup: 120, measure: 'weight' },
        { names: ['pastry flour'], gramsPerCup: 106, measure: 'weight' },
        { names: ['self rising flour', 'self raising flour'], gramsPerCup: 113, measure: 'weight' },
        { names: ['whole wheat flour', 'wholemeal flour', 'wheat flour'], gramsPerCup: 113, measure: 'weight' },
        { names: ['whole wheat pastry flour', 'graham flour'], gramsPerCup: 96, measure: 'weight' },
        { names: ['durum flour'], gramsPerCup: 124, measure: 'weight' },
        { names: ['semolina', 'semolina flour'], gramsPerCup: 163, measure: 'weight' },
        { names: ['spelt flour'], gramsPerCup: 99, measure: 'weight' },
        { names: ['rye flour', 'medium rye flour'], gramsPerCup: 106, measure: 'weight' },
        { names: ['vital wheat gluten'], gramsPerCup: 144, measure: 'weight' },

        // other flours and starches
        { names: ['almond flour', 'ground almonds'], gramsPerCup: 96, measure: 'weight' },
        { names: ['almond meal'], gramsPerCup: 84, measure: 'weight' },
        { names: ['hazelnut flour'], gramsPerCup: 89, measure: 'weight' },
        { names: ['oat flour'], gramsPerCup: 92, measure: 'weight' },
        { names: ['coconut flour'], gramsPerCup: 128, measure: 'weight' },
        { names: ['chickpea flour', 'gram flour', 'besan'], gramsPerCup: 85, measure: 'weight' },
        { names: ['buckwheat flour'], gramsPerCup: 120, measure: 'weight' },
        { names: ['barley flour'], gramsPerCup: 85, measure: 'weight' },
        { names: ['rice flour', 'white rice flour'], gramsPerCup: 142, measure: 'weight' },
        { names: ['brown rice flour'], gramsPerCup: 128, measure: 'weight' },
        { names: ['glutinous rice flour', 'sweet rice flour', 'mochiko'], gramsPerCup: 120, measure: 'weight' },
        { names: ['masa harina'], gramsPerCup: 93, measure: 'weight' },
        { names: ['cornmeal', 'yellow cornmeal'], gramsPerCup: 156, measure: 'weight' },
        { names: ['whole grain cornmeal', 'stone ground cornmeal'], gramsPerCup: 138, measure: 'weight' },
        { names: ['polenta', 'coarse cornmeal'], gramsPerCup: 163, measure: 'weight' },
        { names: ['cornstarch', 'corn starch', 'cornflour'], gramsPerCup: 112, measure: 'weight' },
        { names: ['tapioca starch', 'tapioca flour'], gramsPerCup: 113, measure: 'weight' },
        { names: ['potato starch'], gramsPerCup: 152, measure: 'weight' },
        { names: ['cocoa powder', 'cocoa', 'unsweetened cocoa powder', 'unsweetened cocoa', 'dutch process cocoa powder'], gramsPerCup: 84, measure: 'weight' },
        { names: ['milk powder', 'powdered milk', 'dry milk', 'nonfat dry milk', 'dried milk'], gramsPerCup: 112, measure: 'weight' },

        // grains, cereals and crumbs
        { names: ['rolled oats', 'oats', 'old fashioned oats', 'quick oats', 'quick cooking oats'], gramsPerCup: 89, measure: 'weight' },
        { names: ['steel cut oats'], gramsPerCup: 140, measure: 'weight' },
        { names: ['oat bran'], gramsPerCup: 106, measure: 'weight' },
        { names: ['wheat bran'], gramsPerCup: 64, measure: 'weight' },
        { names: ['wheat germ'], gramsPerCup: 112, measure: 'weight' },
        { names: ['rice', 'white rice', 'long grain rice', 'basmati rice', 'jasmine rice'], gramsPerCup: 198, measure: 'weight' },
        { names: ['cooked brown rice'], gramsPerCup: 170, measure: 'weight' },
        { names: ['quinoa'], gramsPerCup: 177, measure: 'weight' },
        { names: ['cooked quinoa'], gramsPerCup: 184, measure: 'weight' },
        { names: ['pearl barley', 'pearled barley', 'barley'], gramsPerCup: 213, measure: 'weight' },
        { names: ['bulgur', 'bulgur wheat', 'bulghur'], gramsPerCup: 152, measure: 'weight' },
        { names: ['millet'], gramsPerCup: 206, measure: 'weight' },
        { names: ['buckwheat', 'buckwheat groats'], gramsPerCup: 170, measure: 'weight' },
        { names: ['granola'], gramsPerCup: 113, measure: 'weight' },
        { names: ['rice krispies', 'crispy rice cereal', 'puffed rice cereal'], gramsPerCup: 28, measure: 'weight' },
        { names: ['panko', 'panko breadcrumbs', 'panko bread crumbs'], gramsPerCup: 50, measure: 'weight' },
        { names: ['breadcrumbs', 'bread crumbs', 'dry breadcrumbs', 'dried breadcrumbs', 'dry bread crumbs'], gramsPerCup: 112, measure: 'weight' },
        { names: ['fresh breadcrumbs', 'fresh bread crumbs'], gramsPerCup: 84, measure: 'weight' },
        { names: ['graham cracker crumbs'], gramsPerCup: 100, measure: 'weight' },
        { names: ['cookie crumbs'], gramsPerCup: 85, measure: 'weight' },

        // sugars and syrups
        { names: ['sugar', 'granulated sugar', 'white sugar', 'cane sugar', 'granulated white sugar'], gramsPerCup: 198, measure: 'weight' },
        { names: ['caster sugar', 'castor sugar', 'superfine sugar'], gramsPerCup: 190, measure: 'weight' },
        { names: ['brown sugar', 'light brown sugar', 'dark brown sugar', 'muscovado sugar', 'light muscovado sugar', 'dark muscovado sugar', 'dark brown muscovado sugar'], gramsPerCup: 213, measure: 'weight' },
        { names: ['powdered sugar', 'confectioners sugar', 'icing sugar'], gramsPerCup: 113, measure: 'weight' },
        { names: ['demerara sugar'], gramsPerCup: 220, measure: 'weight' },
        { names: ['turbinado sugar', 'raw sugar'], gramsPerCup: 180, measure: 'weight' },
        { names: ['coconut sugar'], gramsPerCup: 154, measure: 'weight' },
        { names: ['cinnamon sugar'], gramsPerCup: 200, measure: 'weight' },
        { names: ['honey'], gramsPerCup: 336, measure: 'weight' },
        { names: ['maple syrup'], gramsPerCup: 312, measure: 'weight' },
        { names: ['molasses'], gramsPerCup: 340, measure: 'weight' },
        { names: ['corn syrup', 'light corn syrup', 'dark corn syrup'], gramsPerCup: 312, measure: 'weight' },
        { names: ['agave', 'agave syrup', 'agave nectar'], gramsPerCup: 336, measure: 'weight' },
        { names: ['jam', 'preserves', 'fruit preserves'], gramsPerCup: 340, measure: 'weight' },
        { names: ['sweetened condensed milk', 'condensed milk'], gramsPerCup: 312, measure: 'weight' },
        { names: ['marshmallow fluff', 'marshmallow creme', 'marshmallow cream'], gramsPerCup: 128, measure: 'weight' },
        { names: ['mini marshmallows', 'miniature marshmallows'], gramsPerCup: 43, measure: 'weight' },

        // fats
        { names: ['butter'], gramsPerCup: 227, measure: 'weight' },
        { names: ['coconut oil'], gramsPerCup: 226, measure: 'weight' },
        { names: ['ghee'], gramsPerCup: 176, measure: 'weight' },
        { names: ['lard'], gramsPerCup: 226, measure: 'weight' },
        { names: ['shortening', 'vegetable shortening'], gramsPerCup: 184, measure: 'weight' },
        { names: ['mayonnaise', 'mayo'], gramsPerCup: 226, measure: 'weight' },

        // spreads and pastes
        { names: ['peanut butter'], gramsPerCup: 270, measure: 'weight' },
        { names: ['almond butter'], gramsPerCup: 272, measure: 'weight' },
        { names: ['tahini', 'tahini paste'], gramsPerCup: 256, measure: 'weight' },
        { names: ['nutella'], gramsPerCup: 298, measure: 'weight' },
        { names: ['hazelnut spread', 'chocolate hazelnut spread'], gramsPerCup: 320, measure: 'weight' },
        { names: ['cookie butter', 'biscoff spread'], gramsPerCup: 288, measure: 'weight' },
        { names: ['lemon curd'], gramsPerCup: 226, measure: 'weight' },
        { names: ['almond paste'], gramsPerCup: 259, measure: 'weight' },
        { names: ['marzipan'], gramsPerCup: 290, measure: 'weight' },
        { names: ['tomato paste'], gramsPerCup: 232, measure: 'weight' },
        { names: ['pesto', 'basil pesto'], gramsPerCup: 224, measure: 'weight' },
        { names: ['pizza sauce'], gramsPerCup: 228, measure: 'weight' },
        { names: ['minced garlic'], gramsPerCup: 224, measure: 'weight' },

        // dairy you'd rather weigh
        { names: ['cream cheese'], gramsPerCup: 227, measure: 'weight' },
        { names: ['sour cream'], gramsPerCup: 227, measure: 'weight' },
        { names: ['creme fraiche'], gramsPerCup: 226, measure: 'weight' },
        { names: ['yogurt', 'plain yogurt', 'greek yogurt', 'yoghurt', 'greek yoghurt'], gramsPerCup: 227, measure: 'weight' },
        { names: ['ricotta', 'ricotta cheese'], gramsPerCup: 227, measure: 'weight' },
        { names: ['mascarpone', 'mascarpone cheese'], gramsPerCup: 227, measure: 'weight' },
        { names: ['cottage cheese'], gramsPerCup: 226, measure: 'weight' },
        { names: ['feta', 'feta cheese', 'crumbled feta', 'queso fresco'], gramsPerCup: 114, measure: 'weight' },
        { names: ['shredded cheese', 'grated cheese', 'cheddar cheese', 'cheddar', 'grated cheddar', 'shredded cheddar',
            'shredded mozzarella', 'grated mozzarella', 'monterey jack', 'swiss cheese', 'gruyere'], gramsPerCup: 113, measure: 'weight' },
        { names: ['parmesan', 'parmesan cheese', 'grated parmesan', 'parmigiano reggiano'], gramsPerCup: 100, measure: 'weight' },
        { names: ['coconut cream', 'cream of coconut'], gramsPerCup: 284, measure: 'weight' },

        // chocolate
        { names: ['chocolate chips', 'semisweet chocolate chips', 'semi sweet chocolate chips', 'dark chocolate chips',
            'milk chocolate chips', 'white chocolate chips', 'chocolate chunks', 'chopped chocolate', 'chopped dark chocolate'], gramsPerCup: 170, measure: 'weight' },
        { names: ['mini chocolate chips'], gramsPerCup: 177, measure: 'weight' },
        { names: ['cacao nibs', 'cocoa nibs'], gramsPerCup: 120, measure: 'weight' },
        { names: ['toffee bits', 'toffee chunks', 'heath bits'], gramsPerCup: 156, measure: 'weight' },

        // nuts and seeds (plain "walnuts"/"pecans" use the chopped weight, as most baking recipes chop them)
        { names: ['almonds', 'whole almonds'], gramsPerCup: 142, measure: 'weight' },
        { names: ['sliced almonds', 'flaked almonds'], gramsPerCup: 86, measure: 'weight' },
        { names: ['slivered almonds'], gramsPerCup: 114, measure: 'weight' },
        { names: ['walnuts', 'chopped walnuts'], gramsPerCup: 113, measure: 'weight' },
        { names: ['whole walnuts', 'walnut halves'], gramsPerCup: 128, measure: 'weight' },
        { names: ['pecans', 'chopped pecans', 'diced pecans'], gramsPerCup: 114, measure: 'weight' },
        { names: ['whole pecans', 'pecan halves'], gramsPerCup: 105, measure: 'weight' },
        { names: ['cashews', 'chopped cashews'], gramsPerCup: 113, measure: 'weight' },
        { names: ['hazelnuts'], gramsPerCup: 142, measure: 'weight' },
        { names: ['macadamia nuts', 'macadamias'], gramsPerCup: 149, measure: 'weight' },
        { names: ['peanuts'], gramsPerCup: 142, measure: 'weight' },
        { names: ['pine nuts'], gramsPerCup: 142, measure: 'weight' },
        { names: ['pistachios', 'pistachio nuts', 'shelled pistachios'], gramsPerCup: 120, measure: 'weight' },
        { names: ['sesame seeds'], gramsPerCup: 142, measure: 'weight' },
        { names: ['sunflower seeds'], gramsPerCup: 140, measure: 'weight' },
        { names: ['pumpkin seeds', 'pepitas'], gramsPerCup: 160, measure: 'weight' },
        { names: ['chia seeds'], gramsPerCup: 148, measure: 'weight' },
        { names: ['flaxseed', 'flax seeds', 'flaxseeds'], gramsPerCup: 140, measure: 'weight' },
        { names: ['flax meal', 'ground flaxseed', 'ground flax'], gramsPerCup: 100, measure: 'weight' },
        { names: ['poppy seeds'], gramsPerCup: 144, measure: 'weight' },

        // dried fruit
        { names: ['raisins', 'golden raisins', 'sultanas'], gramsPerCup: 149, measure: 'weight' },
        { names: ['currants', 'dried currants'], gramsPerCup: 142, measure: 'weight' },
        { names: ['dried cranberries', 'craisins'], gramsPerCup: 114, measure: 'weight' },
        { names: ['dried cherries'], gramsPerCup: 142, measure: 'weight' },
        { names: ['dried apricots'], gramsPerCup: 128, measure: 'weight' },
        { names: ['dried blueberries'], gramsPerCup: 156, measure: 'weight' },
        { names: ['dried apples'], gramsPerCup: 85, measure: 'weight' },
        { names: ['dates', 'chopped dates', 'medjool dates'], gramsPerCup: 149, measure: 'weight' },
        { names: ['dried figs'], gramsPerCup: 149, measure: 'weight' },
        { names: ['shredded coconut', 'sweetened shredded coconut', 'desiccated coconut', 'toasted coconut'], gramsPerCup: 85, measure: 'weight' },
        { names: ['unsweetened shredded coconut'], gramsPerCup: 53, measure: 'weight' },
        { names: ['coconut flakes', 'unsweetened coconut flakes'], gramsPerCup: 60, measure: 'weight' },

        // fruit (by the cup, prepared as the chart says: mashed, sliced, diced...)
        { names: ['mashed banana', 'mashed bananas'], gramsPerCup: 227, measure: 'weight' },
        { names: ['applesauce', 'apple sauce'], gramsPerCup: 255, measure: 'weight' },
        { names: ['pumpkin puree', 'canned pumpkin'], gramsPerCup: 227, measure: 'weight' },
        { names: ['apples', 'sliced apples'], gramsPerCup: 113, measure: 'weight' },
        { names: ['blueberries'], gramsPerCup: 155, measure: 'weight' },
        { names: ['raspberries'], gramsPerCup: 120, measure: 'weight' },
        { names: ['strawberries', 'sliced strawberries'], gramsPerCup: 167, measure: 'weight' },
        { names: ['berries', 'mixed berries'], gramsPerCup: 142, measure: 'weight' },
        { names: ['cranberries'], gramsPerCup: 99, measure: 'weight' },
        { names: ['peaches', 'diced peaches'], gramsPerCup: 170, measure: 'weight' },
        { names: ['pears', 'diced pears'], gramsPerCup: 163, measure: 'weight' },
        { names: ['pineapple', 'diced pineapple'], gramsPerCup: 170, measure: 'weight' },
        { names: ['crushed pineapple'], gramsPerCup: 256, measure: 'weight' },
        { names: ['rhubarb'], gramsPerCup: 130, measure: 'weight' },

        // vegetables (by the cup, chopped/diced unless the name says otherwise)
        { names: ['onion', 'onions', 'diced onion', 'chopped onion'], gramsPerCup: 142, measure: 'weight' },
        { names: ['carrots', 'carrot', 'diced carrots'], gramsPerCup: 142, measure: 'weight' },
        { names: ['grated carrots', 'shredded carrots', 'grated carrot', 'shredded carrot'], gramsPerCup: 99, measure: 'weight' },
        { names: ['celery', 'diced celery'], gramsPerCup: 142, measure: 'weight' },
        { names: ['bell pepper', 'bell peppers', 'red bell pepper', 'green bell pepper', 'yellow bell pepper'], gramsPerCup: 142, measure: 'weight' },
        { names: ['mushrooms', 'sliced mushrooms'], gramsPerCup: 78, measure: 'weight' },
        { names: ['scallions', 'green onions', 'spring onions'], gramsPerCup: 64, measure: 'weight' },
        { names: ['shallots', 'shallot'], gramsPerCup: 156, measure: 'weight' },
        { names: ['leeks', 'leek'], gramsPerCup: 92, measure: 'weight' },
        { names: ['zucchini', 'shredded zucchini', 'grated zucchini', 'courgette'], gramsPerCup: 136, measure: 'weight' },
        { names: ['corn', 'corn kernels', 'sweetcorn'], gramsPerCup: 152, measure: 'weight' },
        { names: ['olives', 'sliced olives'], gramsPerCup: 142, measure: 'weight' },
        { names: ['sun dried tomatoes', 'sundried tomatoes'], gramsPerCup: 170, measure: 'weight' },
        { names: ['mashed potatoes'], gramsPerCup: 213, measure: 'weight' },
        { names: ['mashed sweet potatoes', 'mashed sweet potato'], gramsPerCup: 240, measure: 'weight' },

        // liquids: converted between cups and ml; density only matters for grams <-> ml
        { names: ['water'], gramsPerCup: 237, measure: 'volume' },
        { names: ['milk', 'oat milk', 'almond milk', 'soy milk', 'skim milk', 'semi skimmed milk', '2% milk'], gramsPerCup: 244, measure: 'volume' },
        { names: ['buttermilk'], gramsPerCup: 244, measure: 'volume' },
        { names: ['evaporated milk'], gramsPerCup: 250, measure: 'volume' },
        { names: ['heavy cream', 'double cream', 'whipping cream', 'heavy whipping cream', 'single cream', 'light cream', 'cream'], gramsPerCup: 238, measure: 'volume' },
        { names: ['half and half'], gramsPerCup: 241, measure: 'volume' },
        { names: ['oil', 'vegetable oil', 'olive oil', 'canola oil', 'sunflower oil', 'neutral oil', 'rapeseed oil'], gramsPerCup: 218, measure: 'volume' },
        { names: ['stock', 'broth', 'chicken stock', 'chicken broth', 'vegetable stock', 'vegetable broth', 'beef stock', 'beef broth'], gramsPerCup: 237, measure: 'volume' },
        { names: ['coconut milk', 'canned coconut milk'], gramsPerCup: 241, measure: 'volume' },
        { names: ['lemon juice', 'lime juice', 'orange juice', 'key lime juice', 'apple juice'], gramsPerCup: 245, measure: 'volume' },
        { names: ['vinegar', 'white vinegar', 'apple cider vinegar', 'red wine vinegar'], gramsPerCup: 239, measure: 'volume' },
        { names: ['wine', 'white wine', 'red wine', 'whiskey', 'whisky', 'rum', 'brandy', 'bourbon', 'whiskey or rum'], gramsPerCup: 234, measure: 'volume' },
    ];

    // words that don't change the density, dropped from the front of a name before matching
    // (not "dried", "unsweetened", "cooked" etc. - those do change it, so they're spelled out in names)
    const IGNORED_WORDS = new Set([
        'unsalted', 'salted', 'softened', 'melted', 'cold', 'chilled', 'room', 'temperature', 'organic',
        'fine', 'finely', 'extra', 'virgin', 'pure', 'unbleached', 'fresh', 'frozen', 'full', 'fat',
        'sifted', 'packed', 'lightly', 'firmly', 'whole', 'natural', 'good', 'quality', 'toasted',
        'raw', 'shelled', 'peeled', 'pitted', 'large', 'small', 'medium', 'ripe',
    ]);

    const UNITS = [
        // [pattern, family, amount of the family's base unit (ml or g), metric?]
        [/^(cups?|c\.?)(?![a-z])/, 'volume', ML_PER_CUP, false],
        [/^(tablespoons?|tbsps?\.?|tbs\.?)(?![a-z])/, 'volume', 14.787, false],
        [/^(teaspoons?|tsps?\.?)(?![a-z])/, 'volume', 4.929, false],
        [/^(fl\.?\s*oz\.?|fluid\s+ounces?)(?![a-z])/, 'volume', 29.574, false],
        [/^(millilit(?:er|re)s?|ml)(?![a-z])/, 'volume', 1, true],
        [/^(lit(?:er|re)s?|l)(?![a-z])/, 'volume', 1000, true],
        [/^(pints?|pt)(?![a-z])/, 'volume', 473.176, false],
        [/^(quarts?|qt)(?![a-z])/, 'volume', 946.353, false],
        [/^(kilograms?|kilos?|kg)(?![a-z])/, 'weight', 1000, true],
        [/^(grams?|gr?)(?![a-z])/, 'weight', 1, true],
        [/^(ounces?|oz\.?)(?![a-z])/, 'weight', 28.3495, false],
        [/^(pounds?|lbs?\.?)(?![a-z])/, 'weight', 453.592, false],
    ];

    const FRACTIONS = {
        '½': 1 / 2, '⅓': 1 / 3, '⅔': 2 / 3, '¼': 1 / 4, '¾': 3 / 4, '⅕': 1 / 5, '⅖': 2 / 5, '⅗': 3 / 5,
        '⅘': 4 / 5, '⅙': 1 / 6, '⅚': 5 / 6, '⅛': 1 / 8, '⅜': 3 / 8, '⅝': 5 / 8, '⅞': 7 / 8,
    };
    const FRACTION_CHARS = Object.keys(FRACTIONS).join('');

    // words written before an amount that don't change it enough to matter ("packed ⅔ cup")
    const AMOUNT_PREFIX = /^(scant|heaping|heaped|packed|level|generous|about|approximately|approx\.?|around|rounded|lightly|firmly|~)\s*/;

    // don't bother converting tiny amounts (weighing a teaspoon isn't useful)
    const MIN_ML = 29;
    const MIN_GRAMS = 25;

    // lowercase, no accents/apostrophes/dashes: "Crème Fraîche" -> "creme fraiche",
    // "confectioners' sugar" -> "confectioners sugar", "all-purpose flour" -> "all purpose flour"
    function normalizeName(name) {
        return (name || '')
            .normalize('NFD').replace(/[̀-ͯ]/g, '')
            .toLowerCase()
            .replace(/&/g, ' and ')
            .replace(/['’]/g, '')
            .replace(/[^a-z0-9% ]+/g, ' ')
            .replace(/\s+/g, ' ')
            .trim();
    }

    const byName = new Map();
    INGREDIENT_DENSITIES.forEach(entry => {
        entry.names.forEach(name => byName.set(normalizeName(name), entry));
    });

    // density info for an ingredient name, or null if it isn't in the table
    function lookup(name) {
        let words = normalizeName(name).split(' ').filter(Boolean);
        while (words.length) {
            const entry = byName.get(words.join(' '));
            if (entry) {
                return { measure: entry.measure, gramsPerMl: entry.gramsPerCup / ML_PER_CUP };
            }
            if (!IGNORED_WORDS.has(words[0])) return null;
            words = words.slice(1);
        }
        return null;
    }

    // leading number: "2", "2.5", "1/2", "1 1/2", "½", "1½", "1 ½"
    function parseNumber(text) {
        let m = text.match(/^(\d+)\s+(\d+)\s*\/\s*(\d+)/);
        if (m) return { value: Number(m[1]) + Number(m[2]) / Number(m[3]), rest: text.slice(m[0].length) };

        m = text.match(/^(\d+)\s*\/\s*(\d+)/);
        if (m) return { value: Number(m[1]) / Number(m[2]), rest: text.slice(m[0].length) };

        m = text.match(new RegExp(`^(\\d+)?\\s*([${FRACTION_CHARS}])`));
        if (m) return { value: (m[1] ? Number(m[1]) : 0) + FRACTIONS[m[2]], rest: text.slice(m[0].length) };

        // "7.5" or European "7,5" (a comma followed by 3 digits is a thousands separator, not handled)
        m = text.match(/^(\d+)[.,](\d{1,2})(?!\d)|^\d+(?:\.\d+)?|^\.\d+/);
        if (m) {
            const value = m[2] !== undefined ? Number(`${m[1]}.${m[2]}`) : Number(m[0]);
            return { value, rest: text.slice(m[0].length) };
        }

        return null;
    }

    // a count with no unit, e.g. "3" (eggs) or "1½" -> 1.5; null if there's anything else in it
    function parseCount(amount) {
        let text = String(amount || '').trim().toLowerCase();
        while (AMOUNT_PREFIX.test(text)) text = text.replace(AMOUNT_PREFIX, '');
        const number = parseNumber(text);
        return number && number.value > 0 && number.rest.trim() === '' ? number.value : null;
    }

    // one "number unit" piece, e.g. "½ cup" -> { family: 'volume', base: 118.3 }
    function parseChunk(chunk) {
        let text = chunk.trim().toLowerCase();
        while (AMOUNT_PREFIX.test(text)) text = text.replace(AMOUNT_PREFIX, '');

        const number = parseNumber(text);
        if (!number || !(number.value > 0)) return null;

        const rest = number.rest.trim();
        if (/^[-–—]|^to\s/.test(rest)) return null; // ranges like "2-3 cups" aren't converted

        for (const [pattern, family, base, metric] of UNITS) {
            if (pattern.test(rest)) return { family, base: number.value * base, metric };
        }
        return null; // no unit ("3") or not a measuring unit ("1 can", "2 large")
    }

    // "½ cup plus 2 tablespoons" -> { family: 'volume', base: 147.9, metric: false } (base is ml or g);
    // null if it can't be read
    function parseAmount(amount) {
        const chunks = String(amount || '').split(/\s+(?:plus|\+)\s+/i);
        let family = null;
        let base = 0;
        let metric = true;
        for (const chunk of chunks) {
            const parsed = parseChunk(chunk);
            if (!parsed || (family && parsed.family !== family)) return null;
            family = parsed.family;
            base += parsed.base;
            metric = metric && parsed.metric;
        }
        return family ? { family, base, metric } : null;
    }

    function trimNumber(value, decimals) {
        return String(Number(value.toFixed(decimals)));
    }

    function formatGrams(grams) {
        if (grams >= 1000) return `${trimNumber(grams / 1000, 2)} kg`;
        const step = grams < 10 ? 1 : 5;
        return `${Math.max(step, Math.round(grams / step) * step)} g`;
    }

    function formatMl(ml) {
        if (ml >= 1000) return `${trimNumber(ml / 1000, 2)} l`;
        return `${Math.max(5, Math.round(ml / 5) * 5)} ml`;
    }

    const CUP_FRACTIONS = [[0, ''], [1 / 4, '¼'], [1 / 3, '⅓'], [1 / 2, '½'], [2 / 3, '⅔'], [3 / 4, '¾'], [1, '']];

    // nearest quarter or third of a cup, or tablespoons below ¼ cup
    function formatCups(ml) {
        const cups = ml / ML_PER_CUP;
        if (cups < 0.25) {
            const tbsp = Math.max(1, Math.round(ml / 14.787));
            return `${tbsp} tbsp`;
        }
        let whole = Math.floor(cups);
        let [fraction, glyph] = CUP_FRACTIONS.reduce((best, candidate) =>
            Math.abs(cups - whole - candidate[0]) < Math.abs(cups - whole - best[0]) ? candidate : best);
        if (fraction === 1) {
            whole += 1;
            glyph = '';
        }
        const text = `${whole || ''}${glyph}` || '0';
        return `${text} ${whole + (fraction === 1 ? 0 : fraction) > 1 ? 'cups' : 'cup'}`;
    }

    // The other way to measure an amount, e.g. ("granulated sugar", "2½ cups") -> "≈ 495 g",
    // ("flour", "500 g") -> "≈ 4 cups", ("milk", "2 cups") -> "≈ 475 ml", ("milk", "480 ml") ->
    // "≈ 2 cups". null for unknown ingredients, unreadable or tiny amounts.
    function alternativeAmount(name, amount) {
        const info = lookup(name);
        const parsed = info && parseAmount(amount);
        if (!parsed) return null;

        if (info.measure === 'weight') {
            if (parsed.family === 'volume') {
                return parsed.base >= MIN_ML ? `≈ ${formatGrams(parsed.base * info.gramsPerMl)}` : null;
            }
            // weight given: show cups for people without a scale
            return parsed.base >= MIN_GRAMS ? `≈ ${formatCups(parsed.base / info.gramsPerMl)}` : null;
        }

        // liquids: cups <-> ml
        const ml = parsed.family === 'volume' ? parsed.base : parsed.base / info.gramsPerMl;
        if (ml < MIN_ML) return null;
        return parsed.metric ? `≈ ${formatCups(ml)}` : `≈ ${formatMl(ml)}`;
    }

    // Combines a grocery list total ({ volume: ml, weight: g, count: n }) into the unit you'd shop
    // with: grams for dry goods, ml for liquids. Returns the new totals and whether anything was
    // converted, or null for ingredients not in the table.
    function combineTotals(name, totals) {
        const info = lookup(name);
        if (!info) return null;

        const combined = { ...totals };
        let converted = false;
        if (info.measure === 'weight' && combined.volume) {
            combined.weight = (combined.weight || 0) + combined.volume * info.gramsPerMl;
            delete combined.volume;
            converted = true;
        } else if (info.measure === 'volume' && combined.weight) {
            combined.volume = (combined.volume || 0) + combined.weight / info.gramsPerMl;
            delete combined.weight;
            converted = true;
        }
        return { totals: combined, converted, measure: info.measure };
    }

    return {
        lookup, parseAmount, parseCount, alternativeAmount, combineTotals, formatGrams, formatMl, formatCups, normalizeName,
        table: INGREDIENT_DENSITIES, // exposed for tests
    };
})();

if (typeof module !== 'undefined') module.exports = IngredientConversions;

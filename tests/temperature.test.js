// Frontend unit tests: node --test tests/
const test = require('node:test');
const assert = require('node:assert/strict');

const { addTemperatureConversions, convertTemperature } = require('../static/js/temperature.js');

test('adds the other unit after a temperature', () => {
    const cases = {
        'Preheat the oven to 375°F.': 'Preheat the oven to 375°F (190°C).',
        'Turn on the oven to 200 degrees Celsius': 'Turn on the oven to 200 degrees Celsius (400°F)',
        'Bake at 350F': 'Bake at 350F (175°C)',
        'Heat to 180-200°C': 'Heat to 180-200°C (350–400°F)',
        'Heat to 375 degrees F': 'Heat to 375 degrees F (190°C)',
        'Heat to 350 degrees fahrenheit': 'Heat to 350 degrees fahrenheit (175°C)',
        'Roast at 425 °F then drop to 350°F': 'Roast at 425 °F (220°C) then drop to 350°F (175°C)',
        'Internal temp 165°F': 'Internal temp 165°F (74°C)', // not an oven temp, so not rounded to 5
    };
    for (const [input, expected] of Object.entries(cases)) {
        assert.equal(addTemperatureConversions(input), expected, input);
    }
});

test('assumes Fahrenheit for unitless temperatures above 250', () => {
    assert.equal(addTemperatureConversions('Bake at 350 degrees for 20 min'), 'Bake at 350 degrees (175°C) for 20 min');
    assert.equal(addTemperatureConversions('Bake at 350° until set'), 'Bake at 350° (175°C) until set');
    assert.equal(addTemperatureConversions('Bake at 325-350°'), 'Bake at 325-350° (165–175°C)');
    assert.equal(addTemperatureConversions('Preheat to 400 degrees.'), 'Preheat to 400 degrees (205°C).');
});

test('leaves alone text that already has both units, or is not a temperature', () => {
    for (const text of [
        'Bake at 375°F (190°C) until golden',
        'Heat oven to 220°C/425°F',
        'Heat to 190C, 375F, gas mark 5',
        'Bake at 425°F (200°C) until golden', // the author's own (fan oven) conversion is kept
        'Bake at 180° (350°F)',
        'Rotate the pan 180 degrees',
        'Bake at 250 degrees', // unitless, not above 250
        'Add 2 cups flour, 250 cal, 300 g sugar',
        'No temps here',
    ]) {
        assert.equal(addTemperatureConversions(text), text, text);
    }
});

test('rounds oven temperatures to what ovens use', () => {
    assert.equal(convertTemperature(375, true), 190); // °F -> °C, nearest 5
    assert.equal(convertTemperature(425, true), 220);
    assert.equal(convertTemperature(200, false), 400); // °C -> °F, nearest 25
    assert.equal(convertTemperature(180, false), 350);
    assert.equal(convertTemperature(165, true), 74); // below oven range: nearest degree
});

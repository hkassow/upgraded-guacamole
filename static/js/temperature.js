// Adds the other temperature unit to recipe steps for display, e.g. "Bake at 375°F" ->
// "Bake at 375°F (190°C)". Used by the recipe view and edit box in main.js.
// Also loads in node for tests: node --test tests/

// Matches "375°F", "375 °F", "375F", "375 degrees F", "200 degrees Celsius", "180-200°C",
// and unitless "350°" / "350 degrees" (group 3 undefined).
const TEMPERATURE_REGEX = /\b(\d{2,3})(?:\s*(?:-|–|—|to)\s*(\d{2,3}))?(?:(?:\s*[°º]\s*|\s*degrees?\s+|\s*deg\.?\s*)?(fahrenheit|celsius|centigrade|f|c)\b|\s*[°º]|\s+degrees?\b)/gi;
// unitless temps above this are assumed Fahrenheit, at or below it they're left alone ("rotate 180 degrees")
const UNITLESS_FAHRENHEIT_MIN = 250;
// text allowed between two temperatures that are already a conversion pair, e.g. "375°F (190°C)", "190°C/375°F", "375°F or 190°C"
const TEMPERATURE_PAIR_GAP = /^\s*[(\[\/,]?\s*(?:or\s+)?$/i;

function convertTemperature(value, toCelsius) {
    const converted = toCelsius ? (value - 32) * 5 / 9 : value * 9 / 5 + 32;
    // oven temps round to what ovens use (5°C / 25°F), lower temps (internal/candy) stay precise
    const isOvenTemp = toCelsius ? value >= 250 : value >= 120;
    const step = isOvenTemp ? (toCelsius ? 5 : 25) : 1;
    return Math.round(converted / step) * step;
}

// Appends the other temperature unit to a step for display, e.g. "Bake at 375°F" -> "Bake at 375°F (190°C)".
// Also used for the edit box, so saved steps may contain the conversion; already-paired
// temperatures are skipped so it's never added twice.
function addTemperatureConversions(text) {
    const matches = [...text.matchAll(TEMPERATURE_REGEX)];
    if (!matches.length) return text;

    const isUnitless = (m) => !m[3];
    const isFahrenheit = (m) => isUnitless(m) || m[3].toLowerCase().startsWith('f');
    // a unitless temp next to another temp is treated as a pair since we can't tell its unit
    const isPairedWith = (a, b) =>
        a && b && (isUnitless(a) || isUnitless(b) || isFahrenheit(a) !== isFahrenheit(b)) &&
        TEMPERATURE_PAIR_GAP.test(text.slice(a.index + a[0].length, b.index));

    let result = '';
    let lastIndex = 0;
    matches.forEach((m, i) => {
        const end = m.index + m[0].length;
        result += text.slice(lastIndex, end);
        lastIndex = end;

        // recipe already lists both units
        if (isPairedWith(m, matches[i + 1]) || isPairedWith(matches[i - 1], m)) return;
        if (isUnitless(m) && Number(m[1]) <= UNITLESS_FAHRENHEIT_MIN) return;

        const toCelsius = isFahrenheit(m);
        const low = convertTemperature(Number(m[1]), toCelsius);
        const high = m[2] ? convertTemperature(Number(m[2]), toCelsius) : null;
        const unit = toCelsius ? '°C' : '°F';
        result += ` (${high !== null ? `${low}–${high}` : low}${unit})`;
    });
    return result + text.slice(lastIndex);
}

if (typeof module !== 'undefined') module.exports = { addTemperatureConversions, convertTemperature };

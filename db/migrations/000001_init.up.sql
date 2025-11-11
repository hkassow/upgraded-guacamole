-- =====================================================
-- 000001_create_core_tables.up.sql
-- recipes, ingredients
-- =====================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";


CREATE TABLE recipes (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    title TEXT NOT NULL,
    steps TEXT,
    additional_notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);


CREATE TABLE ingredients (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    season TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);


CREATE TABLE recipe_ingredient (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    recipe_id INT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    ingredient_id INT NOT NULL REFERENCES ingredients(id) ON DELETE CASCADE,
    amount TEXT,
    UNIQUE (recipe_id, ingredient_id)
);


CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Recipes
CREATE TRIGGER trigger_update_recipes
BEFORE UPDATE ON recipes
FOR EACH ROW
EXECUTE PROCEDURE update_updated_at_column();

-- Ingredients
CREATE TRIGGER trigger_update_ingredients
BEFORE UPDATE ON ingredients
FOR EACH ROW
EXECUTE PROCEDURE update_updated_at_column();


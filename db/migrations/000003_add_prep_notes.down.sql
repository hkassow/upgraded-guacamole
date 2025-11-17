ALTER TABLE recipe_ingredient
DROP COLUMN prep_notes;

ALTER TABLE ingredients
ADD CONSTRAINT unique_ingredient_name UNIQUE (name);

CREATE TABLE grocery_tag (
	id SERIAL PRIMARY KEY,
	uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
	ingredient_id INT NOT NULL REFERENCES ingredients(id) ON DELETE CASCADE,
	category TEXT,
	location TEXT
);

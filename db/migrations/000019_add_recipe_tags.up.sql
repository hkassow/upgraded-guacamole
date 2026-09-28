-- tag names are stored lowercase and trimmed (see lib.NormalizeTags), so a plain UNIQUE is enough
CREATE TABLE IF NOT EXISTS tags (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recipe_tags (
    recipe_id INT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    tag_id INT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (recipe_id, tag_id)
);

-- the primary key covers lookups by recipe; this covers "recipes with tag X"
CREATE INDEX IF NOT EXISTS recipe_tags_tag_id_idx ON recipe_tags (tag_id);

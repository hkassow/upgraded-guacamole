CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    display_name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_login TIMESTAMP NOT NULL DEFAULT NOW()
);

ALTER TABLE recipes
ADD COLUMN user_id INT REFERENCES users(id);


/*
CREATE TABLE family (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL
)

CREATE TABLE user_family_join (
    id SERIAL PRIMARY KEY,
    user_id REFERENCES users(id) ON DELETE CASCADE,
    family_id REFERENCES family(id) ON DELETE CASCADE
*/
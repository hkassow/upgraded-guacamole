ALTER TABLE recipe_jobs
ADD COLUMN user_id INT REFERENCES users(id);

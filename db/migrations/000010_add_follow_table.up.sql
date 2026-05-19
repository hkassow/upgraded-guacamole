CREATE TABLE users_follows (
	id SERIAL PRIMARY KEY,
	uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
	followee_id INT REFERENCES users(id),
	follower_id INT REFERENCES users(id),
	created_at TIMESTAMP NOT NULL DEFAULT NOW()
)

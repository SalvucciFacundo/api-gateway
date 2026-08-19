-- +goose Up
-- Initial schema: the users table backing the gateway's Store contract.
-- password_hash is NOT NULL because every user, including the seeded admin,
-- is created with bcrypt credentials.
CREATE TABLE users (
    id            text PRIMARY KEY,
    email         text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS users;

-- name: CreateChirp :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES (
    generate_uuid(), NOW(), NOW(), $1, $2
)
RETURNING *;

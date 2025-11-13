-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1 LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
WHERE is_active = true
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT COUNT(*) FROM users
WHERE is_active = true;

-- name: CreateUser :one
INSERT INTO users (
    email,
    username,
    full_name,
    password_hash
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: UpdateUser :exec
UPDATE users
SET
    full_name = COALESCE($1, full_name),
    updated_at = NOW()
WHERE id = $2;

-- name: UpdateUserPassword :exec
UPDATE users
SET
    password_hash = $1,
    updated_at = NOW()
WHERE id = $2;

-- name: DeactivateUser :exec
UPDATE users
SET
    is_active = false,
    updated_at = NOW()
WHERE id = $1;

-- name: ActivateUser :exec
UPDATE users
SET
    is_active = true,
    updated_at = NOW()
WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: SearchUsers :many
SELECT * FROM users
WHERE
    is_active = true
    AND (
        username ILIKE '%' || $1 || '%'
        OR full_name ILIKE '%' || $1 || '%'
        OR email ILIKE '%' || $1 || '%'
    )
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

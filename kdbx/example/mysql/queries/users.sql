-- name: GetUserByID :one
SELECT * FROM users
WHERE id = ? LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = ? LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = ? LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
WHERE is_active = TRUE
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users
WHERE is_active = TRUE;

-- name: CreateUser :execresult
INSERT INTO users (
    id,
    email,
    username,
    full_name,
    password_hash
) VALUES (
    ?, ?, ?, ?, ?
);

-- name: UpdateUser :exec
UPDATE users
SET
    full_name = COALESCE(?, full_name),
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateUserPassword :exec
UPDATE users
SET
    password_hash = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeactivateUser :exec
UPDATE users
SET
    is_active = FALSE,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: ActivateUser :exec
UPDATE users
SET
    is_active = TRUE,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = ?;

-- name: SearchUsers :many
SELECT * FROM users
WHERE
    is_active = TRUE
    AND (
        username LIKE CONCAT('%', ?, '%')
        OR full_name LIKE CONCAT('%', ?, '%')
        OR email LIKE CONCAT('%', ?, '%')
    )
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

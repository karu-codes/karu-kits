-- name: GetPostByID :one
SELECT * FROM posts
WHERE id = ? LIMIT 1;

-- name: GetPostBySlug :one
SELECT * FROM posts
WHERE slug = ? LIMIT 1;

-- name: ListPosts :many
SELECT * FROM posts
WHERE published = TRUE
ORDER BY published_at DESC
LIMIT ? OFFSET ?;

-- name: ListPostsByUser :many
SELECT * FROM posts
WHERE user_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: CountPosts :one
SELECT COUNT(*) FROM posts
WHERE published = TRUE;

-- name: CountPostsByUser :one
SELECT COUNT(*) FROM posts
WHERE user_id = ?;

-- name: CreatePost :execresult
INSERT INTO posts (
    id,
    user_id,
    title,
    content,
    slug
) VALUES (
    ?, ?, ?, ?, ?
);

-- name: UpdatePost :exec
UPDATE posts
SET
    title = COALESCE(?, title),
    content = COALESCE(?, content),
    slug = COALESCE(?, slug),
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: PublishPost :exec
UPDATE posts
SET
    published = TRUE,
    published_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UnpublishPost :exec
UPDATE posts
SET
    published = FALSE,
    published_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: IncrementViewCount :exec
UPDATE posts
SET view_count = view_count + 1
WHERE id = ?;

-- name: DeletePost :exec
DELETE FROM posts
WHERE id = ?;

-- name: SearchPosts :many
SELECT * FROM posts
WHERE
    published = TRUE
    AND (
        title LIKE CONCAT('%', ?, '%')
        OR content LIKE CONCAT('%', ?, '%')
    )
ORDER BY published_at DESC
LIMIT ? OFFSET ?;

-- name: GetPostWithUser :one
SELECT
    p.*,
    u.username,
    u.full_name as author_name
FROM posts p
INNER JOIN users u ON p.user_id = u.id
WHERE p.id = ?
LIMIT 1;

-- name: ListPostsWithUsers :many
SELECT
    p.*,
    u.username,
    u.full_name as author_name
FROM posts p
INNER JOIN users u ON p.user_id = u.id
WHERE p.published = TRUE
ORDER BY p.published_at DESC
LIMIT ? OFFSET ?;

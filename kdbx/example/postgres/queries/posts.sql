-- name: GetPostByID :one
SELECT * FROM posts
WHERE id = $1 LIMIT 1;

-- name: GetPostBySlug :one
SELECT * FROM posts
WHERE slug = $1 LIMIT 1;

-- name: ListPosts :many
SELECT * FROM posts
WHERE published = true
ORDER BY published_at DESC
LIMIT $1 OFFSET $2;

-- name: ListPostsByUser :many
SELECT * FROM posts
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPosts :one
SELECT COUNT(*) FROM posts
WHERE published = true;

-- name: CountPostsByUser :one
SELECT COUNT(*) FROM posts
WHERE user_id = $1;

-- name: CreatePost :one
INSERT INTO posts (
    user_id,
    title,
    content,
    slug
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: UpdatePost :exec
UPDATE posts
SET
    title = COALESCE($1, title),
    content = COALESCE($2, content),
    slug = COALESCE($3, slug),
    updated_at = NOW()
WHERE id = $4;

-- name: PublishPost :exec
UPDATE posts
SET
    published = true,
    published_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: UnpublishPost :exec
UPDATE posts
SET
    published = false,
    published_at = NULL,
    updated_at = NOW()
WHERE id = $1;

-- name: IncrementViewCount :exec
UPDATE posts
SET view_count = view_count + 1
WHERE id = $1;

-- name: DeletePost :exec
DELETE FROM posts
WHERE id = $1;

-- name: SearchPosts :many
SELECT * FROM posts
WHERE
    published = true
    AND (
        title ILIKE '%' || $1 || '%'
        OR content ILIKE '%' || $1 || '%'
    )
ORDER BY published_at DESC
LIMIT $2 OFFSET $3;

-- name: GetPostWithUser :one
SELECT
    p.*,
    u.username,
    u.full_name as author_name
FROM posts p
INNER JOIN users u ON p.user_id = u.id
WHERE p.id = $1
LIMIT 1;

-- name: ListPostsWithUsers :many
SELECT
    p.*,
    u.username,
    u.full_name as author_name
FROM posts p
INNER JOIN users u ON p.user_id = u.id
WHERE p.published = true
ORDER BY p.published_at DESC
LIMIT $1 OFFSET $2;

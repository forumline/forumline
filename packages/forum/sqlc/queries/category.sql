-- name: ListCategories :many
SELECT id, name, slug, description, sort_order, created_at
FROM categories ORDER BY sort_order;

-- name: GetCategoryBySlug :one
SELECT id, name, slug, description, sort_order, created_at
FROM categories WHERE slug = $1;

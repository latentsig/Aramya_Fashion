-- name: ListCategories :many
SELECT id, slug, name, subtitle, image_key, sort_order
FROM categories
WHERE active = true
ORDER BY sort_order, name;

-- name: ListProducts :many
SELECT p.id, p.sku, p.slug, p.name, p.description, p.category_id,
       c.slug AS category_slug, c.name AS category_name, p.price_paise,
       p.currency, p.image_key, p.status, p.inventory_policy
FROM products p
JOIN categories c ON c.id = p.category_id
WHERE p.status = 'active'
  AND ($1::text IS NULL OR c.slug = $1)
  AND ($2::text IS NULL OR p.name ILIKE '%' || $2 || '%' OR coalesce(p.description, '') ILIKE '%' || $2 || '%')
ORDER BY p.name
LIMIT $3 OFFSET $4;

-- name: GetProductBySlug :one
SELECT p.id, p.sku, p.slug, p.name, p.description, p.category_id,
       c.slug AS category_slug, c.name AS category_name, p.price_paise,
       p.currency, p.image_key, p.status, p.inventory_policy
FROM products p
JOIN categories c ON c.id = p.category_id
WHERE p.status = 'active' AND p.slug = $1;

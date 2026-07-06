-- name: GetInvoice :one
SELECT id, user_id, amount_cents, status, created_at
FROM invoices
WHERE id = $1;

-- name: ListUserInvoices :many
SELECT id, user_id, amount_cents, status, created_at
FROM invoices
WHERE user_id = $1
ORDER BY created_at DESC;

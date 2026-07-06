-- name: GetInvoice :one
SELECT id, user_id, amount_cents, status, created_at
FROM billing.invoices
WHERE id = $1;

-- name: ListUserInvoices :many
SELECT id, user_id, amount_cents, status, created_at
FROM billing.invoices
WHERE user_id = $1
ORDER BY created_at DESC;

# pgkit basic example

This example shows the phase 1 workflow:

```bash
go run ../../cmd/pgkit generate --out db/schema.generated.sql ./schema
go run ../../cmd/pgkit generate --out db/schema.generated.sql --check ./schema
sqlc generate
```

The schema is declared in Go, rendered to reviewable PostgreSQL SQL, and then used as the schema input for `sqlc`.

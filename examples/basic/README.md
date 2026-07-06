# gosqlkit basic example

This example shows the phase 1 workflow:

```bash
go run ../../cmd/gosqlkit generate --out db/schema.generated.sql ./schema
go run ../../cmd/gosqlkit generate --out db/schema.generated.sql --check ./schema
go run ../../cmd/gosqlkit snapshot --out db/schema.snapshot.json ./schema
go run ../../cmd/gosqlkit snapshot --out db/schema.snapshot.json --check ./schema
sqlc generate
```

The schema is declared in Go, rendered to reviewable PostgreSQL SQL, and then used as the schema input for `sqlc`.

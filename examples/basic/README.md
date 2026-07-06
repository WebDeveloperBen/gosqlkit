# gosqlkit basic example

This example shows the schema-layer workflow:

```bash
cd examples/basic
go run ../../cmd/gosqlkit generate
go run ../../cmd/gosqlkit generate --check
go run ../../cmd/gosqlkit snapshot
go run ../../cmd/gosqlkit snapshot --check
sqlc generate
```

The schema is declared in Go, configured via `gosqlkit.yaml`, rendered to
reviewable PostgreSQL SQL, and then used as the schema input for `sqlc`.

# Database Strategy

## Current State

The service uses Ent ORM for persistence. On server startup, it currently performs automatic schema migration in `cmd/server/main.go` via `client.Schema.Create(...)`.

## Supported Drivers

SQLite is the currently utilized and supported database driver.

## Future Migrations

As the schema evolves, this project should transition from startup auto-migration to versioned migration files. Versioned migrations will better support rollback workflows and complex data transformations.

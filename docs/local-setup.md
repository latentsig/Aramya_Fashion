# Local development

## Storefront

Requirements: Node.js compatible with the repository lockfile and npm.

```powershell
cd frontend
npm ci
npm run dev
```

The Vite storefront is served on the address printed by Vite (the existing local workflow uses port 8000). `npm run test` runs the current Vitest suite; `npm run build` creates a production bundle.

## Go API and PostgreSQL catalog milestone

Requirements: Go 1.27 or newer, PostgreSQL 14 or newer, and network access to the Go module proxy on first dependency download.

1. Create a local PostgreSQL database and a least-privilege development role using your local PostgreSQL administration workflow. Do not put the password in source control.
2. In PowerShell, set `DATABASE_URL` to that local connection string. Optional API settings are in `backend/.env.example`; the Go binaries read environment variables directly and do not load `.env` files.
3. From `backend`, apply the migration and seed the source catalog:

```powershell
$env:DATABASE_URL = 'postgres://<local-role>:<local-password>@localhost:5432/aramya?sslmode=disable'
$env:API_ADDR = '127.0.0.1:18080'
go run ./cmd/migrate
go run ./cmd/seed
go run ./cmd/api
```

The seed command expects `database/seeds/catalog.json`, generated from the actual legacy catalog with `node scripts/audit/export-catalog.mjs` from the repository root. Alternatively set `CATALOG_SEED_PATH` / `MIGRATIONS_DIR` to explicit paths if launching commands from another directory. The first migration creates catalog tables and no inventory quantities; the commerce migration adds made-to-measure product variants without stock. Configure `CORS_ALLOWED_ORIGINS` for the frontend origin. Vite proxies `/api` to `127.0.0.1:18080` in local development.

Migrations are applied in lexical order and recorded in `schema_migrations`. For a pre-production local rollback, use `go run ./cmd/migrate -down 0002_commerce`; only the latest migration can be rolled back. The commerce rollback drops the new commerce tables, refuses to collapse inventory when more than one variant has stock for a product, and preserves a single variant's quantities. Use it only for development or after a verified backup. Rolling back `0001_catalog` drops the catalog tables and is destructive.

## Current limitations

The API currently serves read-only categories and products. Customer identity, carts, checkout, orders, payment, R2, appointments, admin, PWA and Android setup are not implemented yet. Razorpay credentials are not needed or used by this milestone. Never use live payment credentials for development.

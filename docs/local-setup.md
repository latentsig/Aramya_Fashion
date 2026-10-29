# Local development

## Storefront

Requirements: Node.js compatible with the repository lockfile and npm.

```powershell
cd frontend
npm ci
npm run dev
```

The Vite storefront is served on the address printed by Vite (the existing local workflow uses port 8000). `npm run test` runs the current Vitest suite; `npm run build` creates a production bundle.

## Go API, PostgreSQL catalog, and customer sessions

Requirements: Go 1.27 or newer, PostgreSQL 14 or newer, and network access to the Go module proxy on first dependency download.

1. Create a local PostgreSQL database and a least-privilege development role using your local PostgreSQL administration workflow. Do not put the password in source control.
2. In PowerShell, set `DATABASE_URL` to that local connection string and generate an independent CSRF signing secret for this environment. Optional API settings are in `backend/.env.example`; the Go binaries read environment variables directly and do not load `.env` files.
3. From `backend`, apply the migration and seed the source catalog:

```powershell
$env:DATABASE_URL = 'postgres://<local-role>:<local-password>@localhost:5432/aramya?sslmode=disable'
$env:API_ADDR = '127.0.0.1:18080'
$bytes = New-Object byte[] 32
[System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
$env:CSRF_SECRET = [Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+','-').Replace('/','_')
go run ./cmd/migrate
go run ./cmd/seed
go run ./cmd/api
```

The seed command expects `database/seeds/catalog.json`, generated from the actual legacy catalog with `node scripts/audit/export-catalog.mjs` from the repository root. Alternatively set `CATALOG_SEED_PATH` / `MIGRATIONS_DIR` to explicit paths if launching commands from another directory. The first migration creates catalog tables and no inventory quantities; the commerce migration adds made-to-measure product variants without stock. Configure `CORS_ALLOWED_ORIGINS` for the frontend origin. Vite proxies `/api` to `127.0.0.1:18080` in local development.

Customer registration, login, logout, and current-session lookup are available under `/api/v1/auth`. The API stores Argon2id password hashes, opaque session-token hashes, and server-verified CSRF hashes. The anonymous CSRF token endpoint uses `CSRF_SECRET`; keep the value private and use a separate random value in each environment. Development cookies default to non-Secure/Lax for localhost; use HTTPS and `SESSION_COOKIE_SECURE=true` outside local development.

Migrations are applied in lexical order and recorded in `schema_migrations`. For a pre-production local rollback, use `go run ./cmd/migrate -down 0002_commerce`; only the latest migration can be rolled back. The commerce rollback drops the new commerce tables, refuses to collapse inventory when more than one variant has stock for a product, and preserves a single variant's quantities. Use it only for development or after a verified backup. Rolling back `0001_catalog` drops the catalog tables and is destructive.

## Current limitations

The API serves catalog reads and the initial customer session lifecycle. Customer-facing login UI, password reset/email verification, Google sign-in, carts, checkout, orders, payment, R2, appointments, admin, PWA and Android setup remain unimplemented. Razorpay credentials are not needed or used by this milestone. Never use live payment credentials for development.

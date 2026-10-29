# Architecture

## Current implementation milestone

The active storefront is the React/Vite app in `frontend/`. The original HTML, CSS, JavaScript, and `dist/` were preserved under `legacy-storefront/` for parity checks. The existing catalog is exported from the legacy JavaScript by `scripts/audit/export-catalog.mjs`; deterministic IDs, SKUs, slugs, prices in paise, categories, and original image paths are generated without inventing stock.

The initial backend is a Go/Gin REST service with PostgreSQL/pgx catalog queries. `backend/cmd/migrate` applies ordered `*.up.sql` migrations transactionally and records versions in `schema_migrations`; `backend/cmd/seed` transactionally upserts the catalog and intentionally creates no stock rows. API health and database readiness are separate. Current public routes are `/health`, `/ready`, `/api/v1/categories`, `/api/v1/products`, and `/api/v1/products/:slug`.

## Target architecture

```text
React + TypeScript + Vite + PWA + Capacitor
                   │ HTTPS / JSON
                   ▼
             Go + Gin API
        ┌───────────┼───────────┐
        ▼           ▼           ▼
 PostgreSQL     Cloudflare R2  Razorpay
```

PostgreSQL is authoritative for catalog, identity, cart, inventory, orders, payments, appointments, and audit history. R2 stores media objects; the database stores object keys and metadata. Razorpay stays in test mode until merchant and operational approvals are complete. No production infrastructure is provisioned by this repository milestone.

## Security and data boundaries

- API configuration is read from environment variables; secrets are not checked into the repository.
- CORS uses an explicit origin allowlist.
- Health does not reveal internal details; readiness checks the database.
- HTTP logs use route templates and exclude query strings and request bodies.
- Prices are integer paise and the seed does not assert historical stock values.
- The initial schema protects historical catalog rows with restrictive foreign keys.

## Not implemented yet

Authentication, customer/cart/order persistence, inventory reservations, payments, webhooks, admin, R2, appointments, metrics, PWA, Android, and cloud deployment remain future milestones. The API does not yet provide checkout and must not be represented as a functioning payment/order platform.

# Architecture

## Current implementation milestone

The active storefront is the React/Vite app in `frontend/`. The original HTML, CSS, JavaScript, and `dist/` were preserved under `legacy-storefront/` for parity checks. The existing catalog is exported from the legacy JavaScript by `scripts/audit/export-catalog.mjs`; deterministic IDs, SKUs, slugs, prices in paise, categories, and original image paths are generated without inventing stock.

The backend is a Go/Gin REST service with PostgreSQL/pgx repositories and explicit service boundaries. `backend/cmd/migrate` applies ordered `*.up.sql` migrations transactionally and records versions in `schema_migrations`; it only rolls back the latest applied migration and runs the paired down SQL in a transaction. `backend/cmd/seed` transactionally upserts the catalog and creates made-to-measure variant identities without creating stock. The commerce migration includes users, hashed sessions and reset tokens, RBAC, addresses, variants, inventory ledger and reservations, carts, orders, payments, refunds, shipments, appointments, custom-order requests, media metadata, idempotency records, and audit logs. API health and database readiness are separate. Public routes include `/health`, `/ready`, `/api/v1/categories`, `/api/v1/products`, and `/api/v1/products/:slug`. Staff routes currently cover studio requests, inventory, catalog products, and categories; catalog mutations use optimistic versions, role checks, CSRF, transactions, and audit logs. Product image management is limited to existing catalog asset references until R2 uploads are implemented.

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
- The commerce rollback refuses to convert inventory if multiple variants for one product have stock; otherwise it preserves legacy product-level on-hand and reserved quantities.

## Not implemented yet

Authentication, customer/cart/order persistence, inventory reservations, payments, webhooks, admin, R2, appointments, metrics, PWA, Android, and cloud deployment remain future milestones. The API does not yet provide checkout and must not be represented as a functioning payment/order platform.

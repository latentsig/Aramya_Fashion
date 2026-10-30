# Architecture

## Current implementation milestone

The active storefront is the React/Vite app in `frontend/`. The original HTML, CSS, JavaScript, and `dist/` were preserved under `legacy-storefront/` for parity checks. The existing catalog is exported from the legacy JavaScript by `scripts/audit/export-catalog.mjs`; deterministic IDs, SKUs, slugs, prices in paise, categories, and original image paths are generated without inventing stock.

The backend is a Go/Gin REST service with PostgreSQL/pgx repositories and explicit service boundaries. `backend/cmd/migrate` applies ordered `*.up.sql` migrations transactionally and records versions in `schema_migrations`; it only rolls back the latest applied migration and runs the paired down SQL in a transaction. `backend/cmd/seed` transactionally upserts the catalog and creates made-to-measure variant identities without creating stock. The commerce migration includes users, hashed sessions and reset tokens, RBAC, addresses, variants, inventory ledger and reservations, carts, orders, payments, refunds, shipments, appointments, custom-order requests, media metadata, idempotency records, and audit logs. The React/Vite public site and PWA and separate Expo React Native mobile client consume the API. Public API routes include health/readiness, catalog, sessions, guest/customer cart, pending checkout orders, customer order history, and studio requests. Staff routes cover studio requests, inventory, catalog products/categories, and order fulfillment. Order state transitions are optimistic, audited and restricted; payment/refund outcomes are provider-owned, not staff-set. Product image management is limited to existing catalog asset references until R2 uploads are implemented.

The API can expose an authenticated Prometheus scrape endpoint when `PROMETHEUS_BEARER_TOKEN` is configured. It records HTTP route-template counts/latencies, PostgreSQL readiness, checkout order-attempt outcomes, and Go runtime metrics. Local Prometheus/Grafana configuration, a provisioned dashboard, alert rules, and a runbook live under `infrastructure/monitoring/`. The media migration manifest records source bytes, SHA-256 hashes, legacy duplicates, and catalog image references. Admin uploads now use private R2 staging and promote only files whose size, MIME type, image signature, and SHA-256 match their intent; live R2 access and the legacy catalog migration remain unverified until an account and buckets are configured.

## Target architecture

```text
React + TypeScript + Vite + PWA       React Native + Expo
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

## Not implemented or not yet verified

- Razorpay test order creation, signature/webhook verification, reconciliation, and refunds. Current checkout only persists pending-payment orders and explicitly reports payment unavailable.
- Live Cloudflare R2 uploads/migration; product images still reference preserved repository assets. The current source inventory is in `docs/asset-migration-manifest.json`. See `docs/r2-media-migration.md` for upload design and configuration.
- Google sign-in, password reset/email verification, customer quotation approval, and notification delivery.
- Local monitoring stack has parsed configuration but has not been run against a live API scrape in this environment. Production metrics ingress, alert delivery, and cloud monitoring still need deployment configuration.
- Android APK/device testing; local Android SDK, ADB, and emulator are unavailable. Mobile authentication and checkout remain unimplemented.
- GCP staging/production deployment and external account provisioning.

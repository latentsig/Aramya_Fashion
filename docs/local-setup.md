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

The cart API is available under `/api/v1/cart`. A first `GET` creates a guest cart and sets a random HttpOnly cart cookie; the database stores only its hash. `POST /items` accepts a product variant ID and quantity, while `PATCH` and `DELETE` target server-issued cart-line IDs. Subtotals come from database prices. Tracked inventory is checked when adding or changing quantity; guest lines merge into an authenticated customer's cart after login and tracked quantities are clamped to remaining stock. The storefront uses these API calls when the live catalog provides variant IDs.

### Studio administration

The `/admin/appointments`, `/admin/custom-orders`, and `/admin/inventory` staff screens read and update PostgreSQL-backed studio requests and variant stock. To create a first development administrator, use the hidden password-prompt procedure in [admin-studio-workflow.md](admin-studio-workflow.md). Never promote a customer from browser code or expose an admin password in shell history. Appointment and custom-request transitions are audited; a custom quotation remains pending customer approval and does not create an order or payment. Inventory corrections require an expected current quantity and a reason, cannot reduce stock below active reservations, and write the inventory ledger and audit event transactionally.

The `/admin/products` and `/admin/categories` screens manage PostgreSQL catalog records. Product and category edits use optimistic version checks and audited transactions. Product variants are updated or deactivated rather than deleted, preserving historical order references. Products and categories can be archived/deactivated. With all six `CLOUDFLARE_R2_*` values configured in the Go API and `VITE_MEDIA_BASE_URL` set to the approved HTTPS delivery origin in `frontend/.env.local`, admins can upload JPEG, PNG, WebP or AVIF images up to 10 MB from the product form. The browser hashes the bytes, obtains a short-lived private upload URL, uploads directly to staging and calls completion. The image becomes selectable only after server-side size, MIME, signature and SHA-256 verification and promotion. Without R2 configured, upload requests return HTTP 503; existing catalog images remain available. These pages use the same admin session and CSRF protections.

The `/admin/orders` screen lists and searches guest/customer orders and displays item snapshots, contact and shipping details, order history, latest payment/refund summary, and shipment tracking. Staff transitions use an expected order version and audited reason. The allowed flow is paid → confirmed → processing → ready to ship → shipped → delivered. A pending-payment order may be cancelled only when it has no unresolved payment attempt. Shipping requires a carrier and tracking number. Staff cannot set payment/refund status through fulfillment; those states require verified provider workflows. Razorpay test-mode REST and signature primitives are documented in [payment workflow](payment-workflow.md), but checkout remains disabled until payment attempts, capture confirmation and durable webhook processing are connected.

Checkout exposes `POST /api/v1/checkout/quote` and idempotent `POST /api/v1/checkout/orders`. The order endpoint snapshots current product and variant data in a PostgreSQL transaction, records the initial `pending_payment` event, and clears the cart. Until shipping/tax policy and Razorpay test credentials are configured, the quote is marked provisional, payment is explicitly unavailable, and the storefront only saves a pending order request—it does not claim a payment or reserve inventory.

Migrations are applied in lexical order and recorded in `schema_migrations`. Roll back only the latest migration at a time. With the current schema, use `go run ./cmd/migrate -down 0003_admin_catalog_version` before `go run ./cmd/migrate -down 0002_commerce`. The category-version rollback removes its optimistic-lock column; the commerce rollback drops commerce tables, refuses to collapse inventory when more than one variant has stock for a product, and preserves a single variant's quantities. Use rollbacks only for development or after a verified backup. Rolling back `0001_catalog` drops catalog tables and is destructive.

## Current limitations

The storefront provides `/login`, `/register`, and `/account` using the session API. Password reset/email verification and Google sign-in are not implemented. Payment initiation/verification, cancellation/refund processing, Cloudflare R2 live setup, appointment notifications, customer quotation approval, PWA, and Android setup remain outstanding. Razorpay credentials are optional and unused by the local pending-order flow; if configured, only test-mode key IDs are accepted. Never use live payment credentials for development.

# Acceptance report — implementation in progress

This report tracks verified milestones. It does not claim full e-commerce acceptance; payment processing, customer-facing auth UI, administration, R2, PWA, Android, monitoring, and cloud deployment are not implemented.

## Verified repository/catalog/API milestone

- The React/TypeScript storefront keeps the original Aramya visual system and the preserved legacy source/assets.
- The deterministic exporter read the original JavaScript catalog and produced **39 products in five expected categories**.
- A disposable PostgreSQL 16.15 cluster was started on `127.0.0.1:55432` for integration verification; the password-protected existing PostgreSQL instance was not modified.
- Applied `0001_catalog` and `0002_commerce`; seeded **5 categories, 39 products, 39 made-to-measure variants, and 0 inventory rows**.
- Repeated seed and migration rollback/reapply preserved catalog counts.
- Inserted a labeled temporary stock fixture in the disposable cluster, verified migration mapped on-hand/reserved quantities to the product variant, and verified rollback refuses ambiguous multi-variant stock. Removed the fixture afterward; inventory returned to zero.
- Started the API against the disposable database on `127.0.0.1:18080` and verified health `ok`, readiness `ready`, 5 categories, 39 products, category/search filtering, product detail, and HTTP 400 for `limit=101`.
- `go test ./...`: passed for API and configuration packages; command/catalog packages compile.
- `npm test`: **5 tests passed**.
- `npm run build`: passed; Vite produced the production assets. Rollup reported removable `@__PURE__` comment warnings in upstream Zod files.
- `git diff --check`: passed on the earlier catalog/API milestone.

## Environment evidence and blockers

- Go modules were downloaded through an explicitly approved network command into ignored project-local caches.
- Existing local PostgreSQL role authentication has no configured password; an isolated temporary cluster was used instead. The temporary server is stopped after testing; its ignored `.tmp-pg/` data directory is retained locally.
- GitHub PR #2, which contains the catalog/API baseline, was reviewed and merged to `main` at merge commit `a50a18b4080ea3b91534fc903dbaa97e5e31e779`.
- Commerce-schema changes merged through PR #3 to `main` at `ce5d800143a6a472cd7a5c3f67ce7935a4e786e5`.
- On `feat/commerce-data-model`, applied the paired catalog/commerce migrations with the actual migration CLI, then tested the commerce down/up path.
- The isolated inventory fixture verified a stock row migrates from product-level to the default variant with both on-hand and reserved quantities preserved. A rollback with two stocked variants for one product was rejected; the fixture and variants were removed afterward.
- Repeated seed after rollback/reapply finished with 5 categories, 39 products, 39 made-to-measure variants, and zero inventory rows.
- Final Go checks: `go test ./...` and `go vet ./...` passed. OpenAPI YAML parsed as version 3.1.0 with five defined paths.
- Final frontend checks: `npm test` passed (5/5); `npm run build` passed. Vite emitted two non-failing upstream Zod pure-annotation warnings.
- No Razorpay, R2, Google OAuth, GCP, or Firebase credentials/accounts were used. No external paid infrastructure was provisioned.

## Customer session API milestone — verified locally and merged

- Added email/password registration and login using Argon2id password hashes, random opaque session cookies, session token hashing at rest, expiration, logout revocation, session rotation on authentication, and current-session lookup.
- Added signed, expiring anonymous CSRF tokens and session-bound CSRF hashes; missing, forged, mismatched, and expired unsafe-request tokens are rejected. Authentication requests are rate limited, bodies are capped, and cookie-authenticated CORS stays restricted to the configured origin allowlist.
- Integration-tested against the isolated PostgreSQL cluster: registration `201`, `/me` `200`, logout `204`, login `200`, wrong password `401`, missing/forged CSRF `403`. PostgreSQL confirmed session token hashes are 32 bytes and customer roles were assigned. Test API and PostgreSQL cluster were stopped afterward.
- `go test -count=1 ./...`, `go vet ./...`, and `git diff --check` passed after the auth implementation. Frontend production build passed after the home custom-order photo was changed to a full-length catalog lehenga (`lehanga1.jpg`).
- Integration coverage currently exercises the principal session/CSRF paths; Google OAuth, reset/verification delivery, and frontend login/account pages remain outstanding.
- PR #4 merged to `main` at `c35db70fb9ae02dce4186637dd1fe6c30613bdbd`.

## Persistent cart milestone — verified locally and merged

- Catalog products now include the active default variant UUID. Guest and authenticated carts use `product_variant_id` and server-issued cart-line IDs; the browser no longer calculates or stores cart identity/totals by product name in localStorage.
- Implemented `GET /api/v1/cart`, `POST /api/v1/cart/items`, `PATCH /api/v1/cart/items/:id`, and `DELETE /api/v1/cart/items/:id`, with a random HttpOnly guest-cart cookie whose SHA-256 hash is stored in PostgreSQL. Guest carts merge on the next cart read after authentication.
- The API computes unit price and subtotal from active database catalog prices. Tracked stock is locked and validated on add/update; a tracked inventory integration check rejected an over-stock add with `409` and clamped a stale guest cart to available quantity during merge.
- The React cart provider uses credentialed API requests and CSRF headers, persists guest carts through the server cookie, and updates cart quantity/removal from API responses. Add-to-bag is disabled until a live product variant is available, avoiding a fake local-only cart operation.
- PostgreSQL integration checks passed for guest cart creation, HttpOnly cookie, add/server subtotal, quantity update, guest-to-customer merge, remove, tracked stock rejection, and merge clamp. Temporary stock fixture was removed.
- `go test -count=1 ./...`, `go vet ./...`, `npm test` (6/6), and `npm run build` passed after the implementation. OpenAPI parses as YAML 3.1.0. The temporary API/database processes were stopped after verification.
- PR #5 merged to `main` at `413e97527c7b0a02c5587c5fc06c979d8e060ef7`.

## Checkout and pending-order milestone — verified locally, current branch

- Added a provisional server quote and transactional pending-payment order creation from the database cart; order items preserve product name, SKU, variant description, quantity and unit price at order time.
- Added idempotency records scoped to the customer or guest cart; repeated requests with the same payload replay the original order, while changed payloads return `409`.
- Order creation locks the cart and current catalog rows, revalidates tracked inventory, records the initial order status event, and clears cart lines in the same PostgreSQL transaction. It does not create a payment attempt, claim payment, or reserve stock.
- Local integration checks passed: quote total from current database price, order `201` in `pending_payment`, order item and initial status event in PostgreSQL, cart cleared, identical replay `200`, and conflicting idempotency payload `409`.
- Quotes remain provisional and the storefront marks online payment unavailable because shipping/tax policy and Razorpay test credentials are not configured. No payment success was simulated.
- `go test -count=1 ./...`, `go vet ./...`, `npm test` (7/7), `npm run build`, OpenAPI YAML parsing, and `git diff --check` passed for this checkout milestone. Build warnings remain limited to upstream Zod annotations.

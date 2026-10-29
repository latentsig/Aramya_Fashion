# Acceptance report — implementation in progress

This report tracks verified milestones. It does not claim full e-commerce acceptance; payment processing, password reset/email verification, Google sign-in, order/shipping administration, new R2 uploads, PWA, Android, monitoring, and cloud deployment are not implemented.

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
- Integration coverage exercises the principal session/CSRF paths; Google OAuth and reset/verification delivery remain outstanding.
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

## Customer auth UI milestone — verified locally, current branch

- Added `/login`, `/register`, and `/account` pages using React Hook Form and Zod validation, HTTP-only session cookies, and the backend's CSRF bootstrap endpoint.
- Successful login and registration store the returned customer in the auth context; logout revokes the session and refreshes the cart context so guest/customer carts follow the session identity.
- The header switches between Sign in and Account based on the current session. Auth errors are shown accessibly; there are no fake Google or password-reset actions.
- Frontend auth flow test covers CSRF-protected login/logout; `npm test` passed 8/8 and `npm run build` passed. Sign-up UI is typechecked and exercises the same registration API flow; database registration was verified in the prior auth API integration milestone.

## Customer order history and homepage media refresh — verified locally, current branch

- Added authenticated `GET /api/v1/orders` and `GET /api/v1/orders/:id` with customer ownership checks, bounded pagination, order-item snapshots, totals, and shipping details. Added `/account/orders` and `/account/orders/:id` views and account navigation; the customer sees only API responses for their own authenticated session.
- Replaced the previously rejected hero clip with portrait-format Pexels video 36591890, whose listing describes a full-length traditional-attire twirl at a historic palace and marks the clip free to use. The hero fits the full portrait frame to avoid cropping the outfit; the existing local catalog poster remains as a fallback. See `docs/media-attributions.md`.
- Replaced the blurry custom-order section thumbnail with the repository’s original `bridal lehanga.jpg` (1715×2410 source, 599,591 bytes), preserving its source under the legacy `image assests/` directory.
- Started the disposable PostgreSQL 16 cluster on `127.0.0.1:55432` and the Go API on `127.0.0.1:18080`. Verified API readiness, live product variant IDs, CSRF bootstrap, guest cart read, and a CSRF-protected cart add through the same Vite `/api` proxy used by the browser. The API response reported a real one-line cart and server-calculated subtotal of 480,000 paise (₹4,800); no payment or order completion was simulated.
- `npm test`: 11/11 tests passed. `npm run build`: passed (Vite reports non-failing upstream Zod pure-annotation warnings). `go test -count=1 ./...` and `go vet ./...` passed. `git diff --check` passed.
- The preview at `http://localhost:8000/` serves the updated React source and proxies API traffic to the local Go API. Restarting the stale Vite process removed the browser’s earlier “bag could not be updated” toast; the refreshed page now loads cleanly.
- Remaining verification: confirm external Pexels playback on the target browser/network and complete owner-versus-other-customer order integration coverage. Razorpay, R2, admin, PWA, Android, monitoring, and cloud deployment remain unimplemented.

## Product details and selected-variant add-to-bag — verified locally, current branch

- Added a React Router `/products/:slug` page and linked live collection cards to it. Product details load from PostgreSQL-backed API data, including the primary image plus any gallery records, actual active variant labels, variant price adjustments, and tracked-stock availability.
- Catalog currently contains one `Made to measure` variant per style and no recorded stock quantities. The detail page shows that real option instead of inventing sizes or stock; order price remains server-calculated by the cart API.
- Changed `GET /api/v1/products/:slug` to include variants and images and documented the `ProductDetail` response in OpenAPI. For catalog entries without separate gallery rows, the persisted product image key is used as the single image.
- Verified against the running PostgreSQL-backed API and browser preview: `/products/the-heirloom-saree` showed the actual name, ₹8,900 catalog price, primary image, `Fit: Made to measure`, and an enabled Add to bag action.
- The product page test exercises the actual component with a live-style API response and asserts the selected variant UUID is submitted through the CSRF-protected cart request. `npm test`: 12/12 passed; `npm run build`, `go test -count=1 ./...`, and `go vet ./...` passed.
- Remaining work: richer descriptions and multiple product gallery media depend on catalog content being entered; size/stock variants must only be introduced with operator-supplied catalog and inventory data.

## Studio appointments and made-to-order inquiries — verified locally, current branch

- Replaced the empty appointment anchor with a shared studio dialog for appointment requests and separate custom-garment quotation requests. Both forms validate contact details, request CSRF protection, and use stable idempotency keys on retry.
- Added PostgreSQL-backed `POST /api/v1/appointments` and `POST /api/v1/custom-order-requests`, bounded validation, guest/customer association, request rate limiting, replay responses, and conflict detection when a key is reused with a different payload. OpenAPI documents both endpoints.
- The dialog reports a request as saved, not as a confirmed visit. Custom work remains a quotation and fitting workflow; this inquiry does not take payment.
- Against the disposable local PostgreSQL 16 instance, verified appointment creation (201), exact retry replay (200 with same record ID), changed-payload key conflict (409), custom-order creation (201), and reuse after the 24-hour idempotency window (201 with a new record). Deleted the disposable test rows and idempotency records after verification.
- `npm test -- --maxWorkers=1 --minWorkers=1`: 14/14 passed. `npm run build`: passed (upstream Zod annotation and bundle-size warnings only). `go test -count=1 ./...`, `go vet ./...`, and `git diff --check` passed.
- Remaining work: staff-facing appointment/custom-request review and notification delivery are not implemented; no email provider is configured. Pexels clip preview plays on its source page, but playback from the local storefront itself still needs a target-browser network check. Payment, R2, admin, PWA, Android, monitoring, and cloud deployment remain unimplemented.

## Saree hero video replacement — verified locally

- Replaced the prior generic traditional-attire footage with Pexels video 7684805. Its listing describes a colorful saree twirl, and the source preview shows the model and drape full length. The media URL was confirmed from Pexels' own download page; the homepage's local browser preview loaded and played it.
- Changed the video from contain to cover so its full vertical frame fills the hero instead of leaving large empty bands. Kept the existing still-image poster fallback and documented the source and license in `docs/media-attributions.md`.
- `npm test -- --maxWorkers=1 --minWorkers=1`: 14/14 passed. `npm run build`: passed; only existing Zod annotation and bundle-size warnings.
- Remaining: confirm final aesthetic preference on the target browser/device; this change does not alter payment, admin, PWA, Android, monitoring, or deployment scope.

## Admin studio request workflow — tested locally

- Added role-gated appointment and made-to-order request queues at `/admin/appointments` and `/admin/custom-orders`, with bounded search/status filters and pagination. Customer sessions receive 403; unauthenticated sessions receive 401.
- Added transactional staff status updates with row locks, expected-status conflict checks, positive integer-paise quotation validation, and an `audit_logs` entry in the same transaction. Admin actions cannot mark a custom request customer-approved or start fulfillment.
- Added `go run ./cmd/seed-admin` as explicit development setup. It reads a process-scoped password input, uses Argon2id, creates a new administrator in one transaction, and refuses to elevate an existing customer. Setup steps and request transition rules are documented in `docs/admin-studio-workflow.md`.
- Disposable PostgreSQL 16 / Go API verification passed: admin queue reads, appointment confirmation, stale-write 409, custom-order review and quotation, transition to `awaiting_approval`, admin approval-bypass rejection, and four persisted actor-attributed audit events. Temporary test user, requests, idempotency rows, and audit entries were cleaned up.
- Frontend admin queue tests passed 3/3 as part of the 17-test suite. Go transition-matrix, route authorization/validation, and seed-admin input tests were added; full Go and production-build results are recorded after the final verification run.
- Remaining at that milestone: broader product/order/inventory/media administration, customer-side quotation approval, notifications, R2, Razorpay, PWA, Android, monitoring, and GCP deployment were not implemented. Inventory management is covered in the verification entry below.

## Admin inventory management — tested locally

- Added `/admin/inventory` and role-gated `GET /api/v1/admin/inventory` with product/category/parent SKU/variant SKU/option search and bounded pagination. The screen shows the inventory policy and on-hand, reserved, and available counts for each variant.
- Added `PATCH /api/v1/admin/inventory/{variantID}` for tracked variants. Operators provide a current expected count, target count, and reason. The repository locks the variant, rejects stale edits and changes below reservations, and writes the inventory movement and audit record in the same transaction. Made-to-order and untracked variants are visible but read-only.
- PostgreSQL 16 integration test passed against the local migrated database: verified parent SKU search, persisted quantity changes, reserved-stock rejection, stale-write conflict, concurrent update serialization (one success and one conflict), exactly two movement/audit rows, and cleanup of temporary product data.
- `go test -count=1 ./...`, `go vet ./...`, frontend tests (19/19), and `npm run build` passed. OpenAPI YAML/reference validation passed. Build emitted upstream Zod annotation and existing large-chunk warnings.
- Remaining: product/category editing, order review and fulfillment, media uploads to R2, customer quotation approval, email notifications, Razorpay, PWA, Android, monitoring, and GCP deployment.

## Admin product and category catalog — verified locally

- Added staff pages at `/admin/products` and `/admin/categories`, linked from the admin landing route and existing studio/inventory navigation.
- Added role-protected Go endpoints to search/filter product records, read product variants, create/update products and variants, list/create/update categories, and select image keys already in catalog metadata.
- Product/category mutations validate on the server, use PostgreSQL transactions and optimistic versions, record actor/request-attributed audit entries, and reject duplicate slugs/SKUs. Variants are updated or deactivated rather than deleted; category deactivation is blocked while active products remain. Public product queries exclude inactive categories.
- Added and applied `0003_admin_catalog_version` on the disposable local PostgreSQL 16 instance. Guarded integration coverage creates and cleans temporary rows while checking create/update/archive, stale edit rejection, category safeguards, and audit rows.
- Tests cover validation, admin route authorization, product creation UI (paise conversion and CSRF), and category deactivation context. Product/category images can only be selected from existing catalog references; the UI does not claim to upload to R2.
- Remaining: product image upload to R2, customer quotation approval, payment, PWA, Android, monitoring, GCP deployment, and full end-to-end acceptance.

## Admin order review and fulfillment — implementation in progress

- Added an admin order search/list and detail endpoint with customer contact/shipping details, line-item snapshots, status history, latest payment/refund summaries, and shipment tracking details.
- Added an optimistic, audited fulfillment transition endpoint. It does not permit staff to mark an order paid/refunded. Pending-payment cancellation rejects unresolved payment attempts and releases active inventory reservations. Shipping requires carrier and tracking number.
- Added the responsive `/admin/orders` page with status/search filters, order detail, payment/refund read-only status, tracking, history and fulfillment actions. The UI uses the admin session, CSRF-protected updates, and real API responses.
- Unit, route, UI and guarded PostgreSQL integration checks have been added. Final full-suite and OpenAPI verification will be recorded after execution.
- Remaining: Razorpay verification/webhooks and refund initiation, order analytics summary, R2, PWA, Android, monitoring, deployment, and full end-to-end acceptance.

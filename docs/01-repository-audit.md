# Repository audit — Aramya Fashion House

Date: 2026-10-29  
Branch: `feat/ecommerce-platform` (created from `main`)  
Baseline: original static storefront; no files have been removed or refactored.

## Inventory

| Route | Purpose | Catalog / section |
|---|---|---|
| `/` (`index.html`) | Home, brand story, collection links, made-to-order CTA, studio location and appointment dialog | Five collection cards; hero campaign image |
| `/sarees.html` | Saree collection | 5 products |
| `/bridal.html` | Bridal and lehengas collection | 5 products |
| `/occasion.html` | Occasionwear collection | 14 products |
| `/menswear.html` | Menswear collection | 9 products |
| `/kids.html` | Children's occasionwear | 5 products |
| `/checkout.html` | Contact and delivery request form with bag summary | Client-calculated estimated subtotal; explicitly no payment |

Product catalog source: `script.js`. The array contains **39 products**. Category counts: sarees 5, bridal 5, occasion 14, menswear 10, kids 5. Prices are integer rupee values in source (not paise); product names, collection labels, and image filenames are present. No descriptions, SKUs, variants, or stock quantities are defined. No stock quantity should be inferred.

## Existing behavior

- Product cards render from a hard-coded `products` array; category cards render from `categories`.
- Cart lines are keyed by product name and quantities live in `localStorage` under `aramya-cart`.
- Cart drawer supports add, increment/decrement, removal, and a displayed subtotal.
- Checkout reads the same local cart and submits customer name, email, phone, delivery address, notes, item list and estimated subtotal to FormSubmit (`formsubmit.co/ajax/hello@aramyafashionhouse.com`).
- Appointment dialog submits name, contact, and interest to the same external FormSubmit endpoint.
- Checkout form code shows a success dialog after an error too, clears the cart in both branches, and can thus report an order request as placed without a successful response. This must not be preserved as payment/order success in the new system.
- Mobile navigation toggles the left navigation at the 620px breakpoint. Product grid goes from 4 columns to 2; collection category cards and dialogs have responsive rules.
- Uses native `<dialog>`, HTML required/type validation, aria labels/status, and escape-to-close. Areas to check in migration: dialog focus restoration, mobile nav expanded state/keyboard behavior, image alt coverage, visible focus, and contrast.

## Assets, build artifacts and dependencies

- 42 image files under `image assests/` (including logo); 42 matching copies under `dist/image assests/`.
- Hash comparison found 42 duplicate SHA-256 groups, each representing the same image in source and `dist`; no other duplicate groups were identified.
- `dist/` contains copies of the static HTML/CSS/JS and assets. No package manager, build configuration, or automated deployment configuration is present in the tracked file inventory.
- Third-party runtime requests: Google Fonts (`DM Sans`, `Playfair Display`) and FormSubmit AJAX. Google Maps is linked from the home location section. No JS package dependencies.
- Asset folder name contains a typo (`image assests`), which appears in page/script references; preserve compatibility until the new asset paths are validated.
- No missing static image references were confirmed by the basic scan. Dynamic template paths were excluded from that static check.

## Baseline visual evidence

The existing local server at `http://localhost:8000/` returned HTTP 200 before this audit. Browser accessibility snapshot confirms the home sections, five collection links, studio details, and footer are present. A browser screenshot capture was requested, but the browser screenshot surface returned a blank image in this run; therefore desktop visual evidence is not considered captured. Tablet and mobile visual screenshots have not yet been captured. Do not claim visual parity until these are collected and reviewed.

## Parity checklist

| Original feature | Migration target | Status |
|---|---|---|
| Announcement bar, brand header, home hero | Shared React layout and home route | Not started |
| Story, collection cards and all five categories | Home / collection routes | Catalog audited; not migrated |
| 38 named products, rupee prices, photos | PostgreSQL seed + API + React product cards | Source extracted; count discrepancy (39 expected) open |
| Add/remove/quantity bag and persisted cart | Variant-keyed backend cart; guest persistence | Legacy behavior audited; not migrated |
| Checkout contact/address form | Validated guest checkout and server-calculated quote | Legacy sends request only; no payment/order persistence |
| Appointment dialog and studio information | Persisted appointments and responsive dialog | Legacy external form audited; not migrated |
| Mobile navigation and responsive collections | Responsive React layout, keyboard accessible | Legacy breakpoint audited; visual checks pending |
| Legacy `dist/` deployment copy | Replace only after migration and verification | Preserve untouched |

## Issues and open blockers

1. Capture desktop/tablet/mobile screenshots using a functioning browser screenshot surface.
2. Confirm PostgreSQL availability before backend/database implementation.
3. No credentials or authorized cloud/payment/storage configuration have been inspected; external integrations remain blocked until valid configuration and accounts are available.

## Implementation progress — milestone 1

Created `frontend/` with a strict TypeScript, React, Vite, React Router, TanStack Query and MUI foundation. Added the original Aramya homepage sections, brand header/footer, central color/font tokens, and retained links to the original static collection pages and image files. The original root-level storefront, `dist/`, and source assets remain untouched. Current React routes are only `/` and a temporary catch-all home view; remaining routes are not implemented yet.

Validation evidence:

- `npm install`: completed; npm reported 3 audit findings (1 moderate, 2 critical). Review advisories and dependency versions before deployment.
- `npm test`: 1 homepage smoke test passed, asserting hero and all five original collection cards.
- `npm run build`: passed; Vite produced a production bundle.
- Dev server: `http://localhost:5174/` returned HTTP 200. Port 5173 was already occupied, so Vite selected 5174.
- Windows sandbox denied esbuild child process creation (`spawn EPERM`); build/test/server commands succeeded when run outside the sandbox with approval.
- Visual viewport screenshot capture remains incomplete: the browser surface returned a blank image during initial capture. Responsive screenshots and overflow checks are outstanding.
- No API, database, credentials, deployment, R2, Razorpay or Android work has been performed in this milestone.

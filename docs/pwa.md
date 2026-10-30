# Progressive web app behavior

The production Vite build generates `manifest.webmanifest` and a Workbox service worker through `vite-plugin-pwa`. The worker uses versioned Workbox precache revisions to update the application shell and remove obsolete precache entries. Development mode deliberately does not register a service worker.

## Offline boundaries

- The application shell and local brand/catalog image assets are precached; video files are excluded.
- Public `GET /api/v1/categories` and `GET /api/v1/products[/:slug]` responses use a short network-first strategy and omit cookies.
- Same-origin product image files use a bounded stale-while-revalidate cache.
- Cart, authentication, account, order, checkout, admin, payment, and mutation requests are not runtime-cached. SPA navigation fallback is denied for API, account, admin, and checkout paths.
- The offline banner says browsing saved styles is available but bag changes, checkout, and payments require a network connection.

## Install and updates

The web manifest uses Aramya's burgundy and paper palette and a compact brand mark in 192px and 512px PNG sizes, with SVG as a scalable option. Browsers that support `beforeinstallprompt` get an install action. A waiting service-worker update is user-triggered; applying it activates the new worker and reloads the page.

## Local verification

Run `cd frontend && npm test` and `npm run build`. The build should produce `frontend/dist/manifest.webmanifest` and `frontend/dist/sw.js`. Browser install prompts and offline behavior must also be checked on HTTPS (or localhost) in a browser that supports installation.

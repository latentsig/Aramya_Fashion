# Aramya Fashion House

Aramya keeps its branded React website/PWA and a separate React Native mobile app. Both clients use the Go API and PostgreSQL catalog; uploaded media is intended for Cloudflare R2.

## Projects

- `frontend/` — React + TypeScript web storefront and installable PWA.
- `mobile/` — React Native + Expo SDK 58 client for Android and iOS.
- `backend/` — Go + Gin REST API.
- `database/` — PostgreSQL migrations, catalog seed, and queries.
- `legacy-storefront/` — preserved original static storefront and design source.

## Development

- Web: `cd frontend; npm install; npm run dev`
- API/database: follow [`docs/local-setup.md`](docs/local-setup.md)
- Android/mobile: follow [`docs/mobile-development.md`](docs/mobile-development.md)
- PWA behavior: [`docs/pwa.md`](docs/pwa.md)
- API monitoring: [`docs/monitoring-runbook.md`](docs/monitoring-runbook.md)
- Hero media licensing: [`docs/media-sources.md`](docs/media-sources.md)

The native app is not a WebView. The web app remains the public website and PWA; React Native provides a true native mobile UI.

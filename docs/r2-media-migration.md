# R2 media migration inventory

The reproducible manifest is [`asset-migration-manifest.json`](asset-migration-manifest.json). Generate it from the repository root with:

```powershell
node scripts/migrate-r2/create-media-manifest.mjs
node scripts/migrate-r2/create-media-manifest.mjs --check
```

The script scans only stable source folders: original `image assests/`, the preserved `legacy-storefront/dist/image assests/`, PWA brand assets, and the two Aramya-branded native app icons. It excludes generated `frontend/public/media`, generated build output, and Expo starter artwork so the manifest remains reproducible and doesn’t propose uploading generated files.

## Current inventory

- 90 source file copies were scanned and SHA-256 hashed.
- They represent 46 unique binaries totaling 4,603,871 bytes; 44 copies are byte-identical duplicates across the preserved legacy distribution and brand icon sets.
- All 39 catalog products and all five category image references resolve to a manifest entry.
- The manifest records byte size, MIME type, SHA-256, canonical source path, duplicate paths, proposed R2 key, and catalog-to-object-key candidates.
- No original files were moved, rewritten, or deleted. The old root and `dist/` asset copies remain intact.

The key candidates are a review aid derived from current catalog filenames and categories. They are not written to PostgreSQL or R2 yet. The local environment has no R2 account ID, access key ID, secret access key, bucket names, or public media domain configured. Until those are configured, uploading, R2 object checksum verification, `media_objects` insertion, `product_images` linking, and public image availability remain unverified and must not be claimed complete.

Before uploading, create separate staging and approved-media R2 buckets/namespaces, keep the source bucket private, configure browser CORS for the exact storefront origins and signed `Content-Type`, upload each manifest candidate without recompressing it, compare R2 object size and SHA-256 to the manifest, then link only verified objects. Any legacy tracked media can be removed only after a later independent review and explicit approval.

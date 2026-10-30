# R2 media migration inventory

## Admin upload workflow

The Go API supports a direct browser-to-R2 upload workflow when all six optional R2 settings in `backend/.env.example` are configured. Use separate private staging and approved-media buckets and an HTTPS delivery base URL for the approved bucket. The API issues a five-minute presigned PUT URL limited to the object key, MIME type and byte length; admin requests use the regular session, CSRF check and rate limits. Only JPEG, PNG, WebP and AVIF files up to 10 MiB are accepted.

After upload, the admin calls the completion endpoint. The backend reads the private staging object, checks the recorded length and Content-Type, hashes the original downloaded bytes with SHA-256, validates the raster signature, copies the verified object to the approved bucket with immutable cache headers, and independently reads and hashes the approved copy. It then records approval and an audit row in PostgreSQL and removes the staging object. A cleanup failure is surfaced and completion can be retried. Do not link an image to a product until its media status is approved. The image bytes remain outside PostgreSQL and the Git repository.

The current R2 S3-compatible API documents that the `x-amz-checksum-algorithm` request header is unsupported, so this workflow does not claim an R2-provided full-object checksum: it computes SHA-256 from the downloaded staging bytes and verifies the promoted copy independently. See [R2 S3 compatibility](https://developers.cloudflare.com/r2/api/s3/api/), [presigned URLs](https://developers.cloudflare.com/r2/api/s3/presigned-urls/) and [Go SDK setup](https://developers.cloudflare.com/r2/examples/aws/aws-sdk-go/).

The admin product editor now drives this flow: it SHA-256 hashes the selected image in the browser, requests a CSRF-protected intent, PUTs the bytes directly to R2, and calls completion. On success it selects the approved object key for the product. Storefront and admin previews resolve keys under `products/`, `brand/` and `collections/` against `VITE_MEDIA_BASE_URL`; legacy keys continue using preserved `/media/<filename>` assets. Configure only the approved delivery domain in frontend configuration; never put R2 API credentials in frontend source.

Local unit tests validate the signing contract and workflow ordering without contacting Cloudflare. No Cloudflare credentials, bucket, or delivery domain are currently available here, so live upload, CORS, CDN delivery, R2 permissions, and production migration have not been verified. Configure exact storefront origins and `Content-Type` in bucket CORS; let the browser set `Content-Length` from the selected file bytes.

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

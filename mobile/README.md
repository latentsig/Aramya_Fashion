# Aramya Fashion House — native mobile app

This is a React Native application built with Expo SDK 58 and Expo Router. It is a native Android/iOS client for the Go API; it is not a WebView wrapper. The separate `frontend/` React app remains the website and installable PWA.

## Run on Android

1. Install Node.js LTS, Android Studio, Android SDK platform/build tools required by Expo SDK 58, and an Android emulator or device.
2. Copy `.env.example` to `.env.local` and set API/media URLs reachable from the device.
3. From this directory, run `npm install`, then `npx expo start`.
4. Press `a` to open the Android emulator, or scan the Metro QR code with a compatible development client.
5. For a native development build, run `npx expo prebuild --platform android` followed by `npx expo run:android`.

For an Android emulator, `10.0.2.2` reaches the development machine's localhost. A physical device needs the computer's LAN address. Local HTTP is enabled only for non-production builds when the API URL begins with `http://`. Production profile builds reject non-HTTPS API URLs.

## Current screens

- Home with Aramya colors, campaign video, and live category/product data.
- Collection browsing with category filters and refresh/error states.
- Product detail with live variants, stock availability, and a real API add-to-bag operation.
- Bag with API-backed quantity and remove actions.

The mobile client uses `credentials: include` and the API's CSRF token for guest bag requests. Verify Android's HTTP-only cookie persistence in an emulator/device test before treating guest-cart continuity as accepted. Customer login, order checkout, Razorpay native checkout, appointments, and made-to-order requests remain future mobile work.

## Media

`EXPO_PUBLIC_MEDIA_BASE_URL` points to the legacy `/media` path for development, then should point to the approved R2 custom domain in production. The original tracked catalog assets are not copied into the mobile bundle. API `imageKey` values that use R2 object keys retain their full object path. The hero video is the licensed Pixabay saree model clip documented in `../docs/media-sources.md`; it is a fashion presentation/dance clip and is not confirmed as a complete 360-degree rotation.

## Build checks

```powershell
npx tsc --noEmit
npm run lint
npx expo export --platform android
npx expo prebuild --platform android
npx expo run:android
```

Generated `android/` output is ignored and reproducible. Never commit signing keys, credentials, or generated APK/AAB files. See `../docs/mobile-development.md` for the environment and build status.

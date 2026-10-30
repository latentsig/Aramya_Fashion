# React Native mobile development

## Architecture

The public website and PWA stay in `frontend/` (React + TypeScript). The Android/mobile client lives in `mobile/` (React Native + TypeScript + Expo SDK 58). Expo Router provides file-based screens and native tabs. Both clients call the same Go `/api/v1` service; the mobile client uses the server's live catalog and variant IDs, and it does not embed product binaries in its repository or app bundle.

The native client currently implements home, collections, product detail, and guest bag operations. Cart mutations use CSRF tokens and include the API cookie jar. Native HTTP-only cookie retention still needs an Android emulator/device test. Native checkout, login/session flows, Razorpay's supported mobile SDK flow, appointments, and custom-order submissions have not been implemented yet.

## Local Windows setup

1. Install Node.js LTS, Android Studio, Android SDK Platform Tools (`adb`), the Expo SDK 58 Android compile/build platforms, and create or start an Android emulator.
2. Start PostgreSQL and the Go API using the repository's root local-development instructions.
3. Start the web frontend (`cd frontend; npm run dev`) so the legacy `/media` assets are served on port 5173.
4. Copy `mobile/.env.example` to `mobile/.env.local` and use `http://10.0.2.2:8080` and `http://10.0.2.2:5173/media` for the standard Android emulator. For a physical phone, replace `10.0.2.2` with the development computer's LAN IP and ensure both services are reachable from the phone.
5. Run `cd mobile; npm install; npx expo start`; press `a` for an attached Android emulator.

The API URL is public configuration, not a secret. Do not place credentials in Expo `EXPO_PUBLIC_*` variables. Local HTTP cleartext support is generated only when the configured API URL starts with `http://` and the build profile is not `production`. Production builds reject non-HTTPS API origins.

When the R2 asset migration is complete, set `EXPO_PUBLIC_MEDIA_BASE_URL` to the approved public delivery domain. R2 keys are resolved as full paths; legacy `image assests/...` keys resolve to the filename beneath the configured media base while the migration is pending.

## Android build

For a local native debug build:

```powershell
cd mobile
npx expo prebuild --platform android
npx expo run:android
```

The debug APK is produced under ignored `mobile/android/app/build/outputs/apk/debug/`. The release AAB should be built with the documented EAS or local Gradle release workflow after signing configuration is set up. Never commit keystores or passwords. `expo export --platform android` validates the JavaScript/native-module bundle but does not produce an APK.

## Payment and authentication requirements

Razorpay is not enabled in the mobile client yet. Select and verify the provider's official Android SDK or redirect flow for this React Native environment before adding payment UI. Keep server-side payment order creation, signature verification, authoritative webhooks, and test credentials as described in `payment-workflow.md`. Do not open an order as paid from the app response alone.

Cookie-based customer authentication needs an explicit mobile session design and native callback review. Do not copy HTTP-only session cookies into JavaScript storage. Guest bag requests currently rely on `credentials: include`; verify that Android persists the guest cart and CSRF cookies across requests on a real Android runtime.

## Current machine verification limits

The available environment has Java 24, but no `adb`, `ANDROID_HOME`, or `ANDROID_SDK_ROOT` is available on PATH. Therefore, a local APK build and emulator/device test cannot be claimed until Android SDK Platform Tools and SDK platforms are installed/configured. Expo config, TypeScript, lint, and Android bundle export checks can still run here.

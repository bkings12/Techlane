# TechLane Android

Three separately installable Jetpack Compose apps sharing `:mobile-core` (theme, secure tokens, HTTP helpers, camera/QR, print/share).

| Module | Application ID | APK |
|--------|----------------|-----|
| `:app` (staff) | `com.techlane.ops` | `app/build/outputs/apk/debug/app-debug.apk` |
| `:customer` | `com.techlane.customer` | `customer/build/outputs/apk/debug/customer-debug.apk` |
| `:supplier` | `com.techlane.supplier` | `supplier/build/outputs/apk/debug/supplier-debug.apk` |

Emulator API base defaults to `http://10.0.2.2:8080/api/v1` in each module’s `BuildConfig.API_BASE` (debug).

## Features

### Staff (`:app`)
- Login against Go `/api/v1/auth/login` + device registration (`POST /devices/register`)
- Jobs (estimates, split payments, STK reconcile, receipt print), cash handovers/refunds, branch pickup
- Offline outbox: draft repair, note, part request, provisional cash, photo attachment
- Sync center + periodic flush
- Camera QR / barcode scan (CameraX + ML Kit) and repair photo capture

### Customer (`:customer`)
- Phone OTP sign-in (`/customer/auth/otp/*`)
- Repair list/detail, estimate approve/reject, M-Pesa STK pay
- Receipt print/share, warranty view/claim

### Supplier (`:supplier`)
- Invite accept + email/password login (`/supplier/auth/*`)
- Request queue, quote/decline, mark ready, issue with ZXing collection QR
- Credit voucher print/share, credit ledger

## Build

```bash
cd ~/TechLane/apps/android
./gradlew :app:assembleDebug :customer:assembleDebug :supplier:assembleDebug
```

### Release builds & API base

Release APKs read `BuildConfig.API_BASE` from the release build type (default `https://api.techlane.co.ke/api/v1`). Override at build time with Gradle property `apiBase`:

```bash
./gradlew :app:assembleRelease -PapiBase=https://your-host.example/api/v1
```

### Signing (release)

1. Create a keystore (once):

```bash
keytool -genkey -v -keystore techlane-release.jks -keyalg RSA -keysize 2048 -validity 10000 -alias techlane
```

2. Add to `~/.gradle/gradle.properties` (do not commit secrets):

```properties
TECHLANE_RELEASE_STORE_FILE=/path/to/techlane-release.jks
TECHLANE_RELEASE_STORE_PASSWORD=...
TECHLANE_RELEASE_KEY_ALIAS=techlane
TECHLANE_RELEASE_KEY_PASSWORD=...
```

3. Wire `signingConfigs` in each app module’s `build.gradle.kts` when ready to ship; until then `./gradlew assembleRelease` produces unsigned release APKs suitable for sideload testing.

## Run on this laptop

```bash
# Terminal 1 — API
cd ~/TechLane
docker compose -f deploy/docker-compose.yml up -d postgres
./scripts/migrate-and-run.sh

# Terminal 2 — emulator (Pixel 6 / Android 14)
export ANDROID_HOME="$HOME/Android/Sdk"
export PATH="$ANDROID_HOME/emulator:$ANDROID_HOME/platform-tools:$PATH"
emulator -avd TechLane_Pixel_6

# Install all three APKs side-by-side
adb install -r app/build/outputs/apk/debug/app-debug.apk
adb install -r customer/build/outputs/apk/debug/customer-debug.apk
adb install -r supplier/build/outputs/apk/debug/supplier-debug.apk
```

Or open `~/TechLane/apps/android` in Android Studio and pick the `app`, `customer`, or `supplier` run configuration.

### Demo users

**Staff**
- `owner@techlane.local` / `password`
- `tech@techlane.local` / `password`
- `cashier@techlane.local` / `password`

**Customer**
- Use a repair customer phone (e.g. the number on an intake job).
- OTP: configure BlessedTexts under web-ops **Settings → SMS (OTP)** (owner). Codes are never logged; SMS must be configured to deliver OTP.

**Supplier**
- Seeded contact: `supplier@techlane.local` / `password` (after migrations seed Default Screens supplier contact)
- Or accept an invite token created in web-ops → Suppliers → Invite supplier contact

## Camera on the emulator

In AVD settings, set **Camera** to **VirtualScene** (or Webcam). First Scan visit will ask for camera permission.

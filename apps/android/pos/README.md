# TechLane POS (`:pos`)

A clean-sheet Android app for the counter, built on Kotlin + Jetpack Compose +
Material 3 with a custom TechLane design system.

It installs alongside the existing `:app` (`com.techlane.ops`) rather than
replacing it, so the shop keeps a working till while this one grows. Change
`applicationId` in `build.gradle.kts` when it is ready to take over.

## What's in it today

- **Jobs** — the technician board and workspace: filters, search, diagnosis,
  approval gate, parts, photos, timeline. See "Jobs module" below.
- **Scan** — opens a job from the QR printed on its intake slip.
- **Quick Charge** (Home) — type an amount, optionally say what it's for (a
  stock item or a service; both optional), send an M-Pesa STK prompt or cash.
- **STK wait** — a blocking, honest status screen that stays until the payment
  is confirmed, refused, or explicitly parked. Never invents an outcome.
- **Sales** — every prompt this phone sent, including unresolved ones.
- **Auth** — password + MFA, plus fingerprint sign-in.
- **Settings** — branch/stock location, catalog sync, biometrics, theme, sign out.

## Jobs module

Bottom navigation is Home · Jobs · Scan · Sales · More.

### Status vocabulary

The board speaks the technician's language but stores the backend's values.
`intake` is shown as "Received", `in_progress` as "On bench", and so on — the
labels live on `JobStatus`, the wire values never change. `JobStatusTest` pins
the whole table (and the allowed transitions) against
`internal/repair/status.go`, so backend drift fails a test rather than
surfacing as an option the server refuses.

Two board filters are **derived, not stored**:

- *Awaiting approval* — open, with no work authorization. That is exactly the
  condition `ChangeStatus` refuses bench work on.
- *My jobs* — a technician filter.

There is no `diagnosing` or `waiting_customer_approval` status, because the
database and the web console do not have them.

### The authorization rule

`internal/repair/service.go` already refuses `intake|diagnosed → in_progress`
without an authorization (`ErrWorkNotAuthorized`). The app mirrors the
*condition* so it can explain the block before the technician hits it, but it
never decides: the status sheet offers "send estimate" or "record approval" and
has no bypass. Recorded approvals go through `POST /repairs/{id}/authorize-work`
with the channel and who took it written into the audit note.

### Offline

Room is the source of truth the UI reads; the network only fills it. Every
mutation is written locally and appended to `job_outbox`, then drained oldest-first
by `JobSyncWorker`. Order is preserved because a status change and the note that
justifies it must reach the server the same way round. A 4xx (a transition the
server won't take) stops retrying and surfaces via `observeStuckActions()`
instead of looping forever or being dropped.

### Timeline

Composed client-side in `JobDetail.timeline()` from status events, notes,
photos, estimates and parts. The API keeps those in separate tables and exposes
no single feed; adding one would have been a backend concept the web console
does not have.

## Missing backend support (identified, not invented)

- **Photo `kind` and `caption`.** `repair.repair_attachments` stores only file
  name, content type, size and audit columns. `PhotoMetadata` encodes both into
  the file name (`tl~kind~caption~millis.jpg`) — reversible, visible to the web
  console, and survives reinstall. To fix properly: add `kind` and `caption`
  columns and expose them on `RepairAttachment`; the codec then collapses to a
  plain file name with no change above it.
- **Job/device barcodes.** Only the intake-slip QR
  (`techlane://repair-pickup/<CODE>`) actually exists. `ScanPayloads` recognises
  it, accepts a hand-typed `PK-` code and a 15-digit IMEI, and passes anything
  else to search rather than coupling to a format nobody has agreed.
- **Timeline feed.** No endpoint; composed on the client as above.

## Deliberately not built

- **Procurement.** Parts here are repair sale lines
  (`/repairs/{id}/sale-lines`) — the rows the job is billed from. Supplier part
  requests stay on the web console.

## Architecture

```
core/designsystem   tokens (color, type, spacing, shape) + reusable components
core/util           money formatting, MSISDN normalisation
data/remote         Retrofit API, DTOs, auth header + refresh interceptors
data/local          Room: catalog cache, service menu, charge ledger
data/session        EncryptedSharedPreferences tokens, DataStore preferences
data/security       BiometricVault (Keystore-backed refresh-token unlock)
data/repository     Auth, Shop, Charge
domain/model        ChargeTarget, StkStage, M-Pesa result mapping
feature/*           screen + ViewModel pairs (StateFlow, unidirectional)
navigation          NavHost, bottom-tab shell
sync                WorkManager catalog refresh
di                  Hilt modules
```

### How a charge resolves

`ChargeRepository.charge()` returns a `Flow<StkStage>` that always ends on
`Paid`, `Failed`, or `TimedOut`. `POST /pos/checkout` creates the sale and asks
for the prompt, then the loop:

1. polls `GET /payments/{id}` every 2s — this sees the Daraja callback the
   moment it lands, which is the normal path;
2. every 4th tick fires `POST /payments/{id}/mpesa/reconcile`, which forces an
   STK Query at Safaricom and rescues a prompt whose callback never arrived.
   That endpoint is owner/manager-gated, so a 403 just disables the nudge and
   polling continues;
3. on settlement, completes the sale if the webhook hasn't already (stock must
   not stay un-deducted);
4. after 120s, gives up *watching* and reports the prompt as unconfirmed —
   never as failed, because M-Pesa can still settle it afterwards.

`verify()` re-checks from the local charge record, so "Check again" survives the
app being killed.

### Biometrics

No password or PIN is stored. Enrolment encrypts the **refresh token** with an
AES key in the Android Keystore marked `setUserAuthenticationRequired(true)` and
`setInvalidatedByBiometricEnrollment(true)`. Unlock decrypts it behind a
fingerprint touch and exchanges it at `/auth/refresh`; the server still decides
whether the session is valid, so a revoked account cannot get back in.

## Build

```bash
# from apps/android
./gradlew :pos:assembleDebug
./gradlew :pos:assembleRelease
./gradlew :pos:testDebugUnitTest

# point at a LAN or staging API
./gradlew :pos:assembleDebug -PapiBase=http://192.168.1.20:8080/api/v1/
```

Defaults: debug `http://10.0.2.2:8080/api/v1/` (emulator → host),
release `https://api.techlane.co.ke/api/v1/`.

APKs land in `pos/build/outputs/apk/`.

## Not wired up yet

- **Release signing** uses the debug key. Swap in a real upload key before Play.
- **Push** — drop `google-services.json` into `pos/` and the Firebase plugin
  applies itself; `PosMessagingService` is already registered. Without the file
  Firebase never initialises and nothing runs.
- **CameraX / ML Kit** are on the dependency list for the scanner that the
  repairs module will need; no scanner screen ships yet.

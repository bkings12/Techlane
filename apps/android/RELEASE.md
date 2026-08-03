# TechLane POS — Android release process

Covers `:pos` (`com.techlane.pos`, the staff app). `:app`, `:customer` and
`:supplier` currently ship signed with the debug key via
`scripts/publish-ops-apk.sh`-style flows; this document does not change that,
though the same keystore mechanism here would work for them too.

## The release keystore

**There is one production keystore for TechLane POS, and it must never
change.** Android refuses to install an update signed by a different key than
the one already on a phone — the only recovery from a lost keystore is asking
every technician to uninstall and reinstall, losing whatever was cached
offline at the time. Treat it like the master key to every phone in the shop,
because that is what it functionally is.

### Generating it (once)

```bash
keytool -genkeypair -v \
  -keystore techlane-pos-release.jks \
  -alias techlane \
  -keyalg RSA -keysize 4096 -validity 10950 \
  -storetype PKCS12
```

`-validity 10950` is 30 years — long enough that expiry is never the reason a
release fails. You'll be prompted for a store password, a key password, and
the certificate's distinguished name (organisation: TechLane).

### Storing it

- Keep the `.jks` **and** its passwords in a password manager or secrets vault
  the whole ops team can reach — not on one person's laptop.
- Keep an offline backup (encrypted USB drive, printed recovery sheet in a
  safe) in case the vault itself is unavailable.
- The file and its passwords are gitignored (`apps/android/*.jks`,
  `apps/android/keystore.properties`) and must stay that way. Never paste a
  password into a commit message, a PR description, or this file.
- For CI, base64-encode the `.jks` into a GitHub Actions secret (see below) —
  do not commit it, encrypted or otherwise, to the repository.

### Local developer setup

```bash
cp apps/android/keystore.properties.example apps/android/keystore.properties
# edit apps/android/keystore.properties with the real storeFile path and passwords
```

`./gradlew :pos:assembleRelease` then signs with the production key
automatically. With no `keystore.properties` and no environment variables set,
the release build still succeeds but is signed with the **debug** key and
prints a warning — useful for a local smoke build, never for distribution.

### Credential precedence

`pos/build.gradle.kts` reads, in order:

1. Gradle properties: `-PTECHLANE_KEYSTORE_PATH=... -PTECHLANE_KEYSTORE_PASSWORD=... -PTECHLANE_KEY_ALIAS=... -PTECHLANE_KEY_PASSWORD=...`
2. Environment variables of the same names (what CI uses)
3. `apps/android/keystore.properties` (`storeFile`, `storePassword`,
   `keyAlias`, `keyPassword` — a developer's local copy)

## CI secrets

The release workflow (`.github/workflows/android-release.yml`) expects these
repository secrets:

| Secret | Contents |
|---|---|
| `TECHLANE_KEYSTORE_BASE64` | `base64 -w0 techlane-pos-release.jks` |
| `TECHLANE_KEYSTORE_PASSWORD` | keystore password |
| `TECHLANE_KEY_ALIAS` | key alias (`techlane`, if generated as above) |
| `TECHLANE_KEY_PASSWORD` | key password |

The workflow decodes the base64 secret to a temp file for the duration of the
build only; it is never written into the repository or a build artifact.

## Versioning

Both `versionCode` and `versionName` live in `pos/build.gradle.kts`.

- `versionCode` — plain integer. Increment by exactly 1 every release. Never
  reuse or decrease it; Android treats a lower/equal code as "not an update".
- `versionName` — semver (`MAJOR.MINOR.PATCH`) shown to staff, in the app's
  About screen (once one exists), and on the download page.

To cut a release:

1. Bump both fields in `pos/build.gradle.kts`.
2. Commit that change.
3. Tag it: `git tag pos-v1.0.1 && git push origin pos-v1.0.1`.
4. The tag triggers `.github/workflows/android-release.yml`, which builds,
   signs, verifies, renames, and attaches the APK to a GitHub Release.

## Verifying a signature by hand

```bash
$ANDROID_HOME/build-tools/<version>/apksigner verify --verbose --print-certs \
  apps/android/pos/build/outputs/apk/release/techlane-pos-v1.0.0-release.apk
```

Confirms the APK verifies and prints the signer certificate — check the
fingerprint matches the production keystore, not a debug one.

## Download page

`apps/web-ops/public/download/index.html` (mirrored to the other three
`web-*` apps and `design-tokens/brand/download.html`) links the TechLane Staff
card straight to the GitHub Release asset for the current tag, so publishing a
release is what makes the download page's link resolve — no separate step
updates the URL by hand for a tagged release. `scripts/publish-pos-apk.sh`
remains available for pushing a build to R2/`web-dist` outside of the tag flow
(matching how `scripts/publish-ops-apk.sh` already works for the Ops app).

#!/usr/bin/env bash
# One-shot Ops APK release:
#   bump version → build → upload R2 → register /app-version → update download page → mirror to prod
#
# Usage:
#   ./scripts/publish-ops-apk.sh              # bump patch (0.1.17 → 0.1.18), publish everything
#   ./scripts/publish-ops-apk.sh --no-bump    # rebuild/re-upload current versionCode
#   NOTES="cash fix" ./scripts/publish-ops-apk.sh
#
# Secrets: prefer /opt/techlane/deploy/.env.r2 on the server (script scps APK there and uploads).
# Local fallback: source .env.r2 in the repo root (gitignored). Never commit secrets.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

BUMP=1
for arg in "$@"; do
  case "$arg" in
    --no-bump) BUMP=0 ;;
    -h|--help)
      sed -n '2,14p' "$0"
      exit 0
      ;;
  esac
done

PUBLISH_SSH="${PUBLISH_SSH:-root@37.60.233.73}"
PUBLISH_SSH_KEY="${PUBLISH_SSH_KEY:-$HOME/.ssh/contabo_gitlab}"
SSH=(ssh -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new)
SCP=(scp -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new)
if [[ -f "$PUBLISH_SSH_KEY" ]]; then
  SSH+=(-i "$PUBLISH_SSH_KEY")
  SCP+=(-i "$PUBLISH_SSH_KEY")
fi

API_BASE="${API_BASE:-https://api.techlane.co.ke/api/v1}"
GRADLE_FILE="apps/android/app/build.gradle.kts"

# --- optional local R2 env (gitignored) ---
if [[ -f "$ROOT/.env.r2" ]]; then
  # shellcheck disable=SC1091
  set -a && source "$ROOT/.env.r2" && set +a
fi

bump_version() {
  local code name major minor patch
  code="$(grep -E 'versionCode\s*=' "$GRADLE_FILE" | head -1 | grep -oE '[0-9]+')"
  name="$(grep -E 'versionName\s*=' "$GRADLE_FILE" | head -1 | sed -E 's/.*"([^"]+)".*/\1/')"
  IFS=. read -r major minor patch <<<"$name"
  major="${major:-0}"
  minor="${minor:-0}"
  patch="${patch:-0}"
  code=$((code + 1))
  patch=$((patch + 1))
  name="${major}.${minor}.${patch}"
  sed -i -E "s/versionCode = [0-9]+/versionCode = ${code}/" "$GRADLE_FILE"
  sed -i -E "s/versionName = \"[^\"]+\"/versionName = \"${name}\"/" "$GRADLE_FILE"
  echo "$code $name"
}

if [[ "$BUMP" == "1" ]]; then
  read -r VERSION_CODE VERSION_NAME <<<"$(bump_version)"
  echo "==> Bumped to ${VERSION_NAME} (versionCode ${VERSION_CODE})"
else
  VERSION_CODE="$(grep -E 'versionCode\s*=' "$GRADLE_FILE" | head -1 | grep -oE '[0-9]+')"
  VERSION_NAME="$(grep -E 'versionName\s*=' "$GRADLE_FILE" | head -1 | sed -E 's/.*"([^"]+)".*/\1/')"
  echo "==> Using existing ${VERSION_NAME} (versionCode ${VERSION_CODE})"
fi

APK_NAME="techlane-ops.apk"
LOCAL_APK="apps/android/app/build/outputs/apk/release/app-release.apk"
NOTES="${NOTES:-Ops ${VERSION_NAME}}"
R2_PUBLIC_BASE_DEFAULT="https://pub-c5cbd20e1eff426bb9379d8f58fca06f.r2.dev"

echo "==> Building Ops ${VERSION_NAME}"
(cd apps/android && ./gradlew :app:assembleRelease -PapiBase="$API_BASE")
test -f "$LOCAL_APK"

# --- upload + register on the server (secrets stay there) ---
REMOTE_DIR="/tmp/techlane-apk-publish-$$"
echo "==> Shipping APK to ${PUBLISH_SSH}"
"${SSH[@]}" "$PUBLISH_SSH" "mkdir -p ${REMOTE_DIR} /opt/techlane/web-dist/web-ops/downloads /opt/techlane/web-dist/web-ops/download"
"${SCP[@]}" "$LOCAL_APK" "${PUBLISH_SSH}:${REMOTE_DIR}/${APK_NAME}"

echo "==> Upload to R2 + register app_releases on server"
"${SSH[@]}" "$PUBLISH_SSH" \
  "VERSION_CODE='${VERSION_CODE}' VERSION_NAME='${VERSION_NAME}' NOTES=$(printf %q "$NOTES") REMOTE_DIR='${REMOTE_DIR}' APK_NAME='${APK_NAME}' bash -s" <<'REMOTE'
set -euo pipefail
ENV_FILE=/opt/techlane/deploy/.env.r2
if [[ ! -f "$ENV_FILE" ]]; then
  echo "missing $ENV_FILE — create it with R2_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY / R2_BUCKET / R2_PUBLIC_BASE" >&2
  exit 1
fi
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a
R2_BUCKET="${R2_BUCKET:-techlane}"
R2_PUBLIC_BASE="${R2_PUBLIC_BASE:?R2_PUBLIC_BASE required in .env.r2}"
R2_ENDPOINT="${R2_ENDPOINT:-https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com}"
R2_PUBLIC_BASE="${R2_PUBLIC_BASE%/}"
DOWNLOAD_URL="${R2_PUBLIC_BASE}/downloads/${APK_NAME}?v=${VERSION_NAME}"

python3 - <<PY
import os, sys
try:
    import boto3
    from botocore.config import Config
except ImportError:
    import subprocess
    subprocess.check_call([sys.executable, "-m", "pip", "install", "--break-system-packages", "-q", "boto3"])
    import boto3
    from botocore.config import Config

s3 = boto3.client(
    "s3",
    endpoint_url=os.environ["R2_ENDPOINT"],
    aws_access_key_id=os.environ["R2_ACCESS_KEY_ID"],
    aws_secret_access_key=os.environ["R2_SECRET_ACCESS_KEY"],
    region_name="auto",
    config=Config(signature_version="s3v4"),
)
apk = os.path.join(os.environ["REMOTE_DIR"], os.environ["APK_NAME"])
bucket = os.environ["R2_BUCKET"]
ver = os.environ["VERSION_NAME"]
name = os.environ["APK_NAME"]
extra = {
    "ContentType": "application/vnd.android.package-archive",
    "CacheControl": "public, max-age=300",
}
for key in (f"downloads/{name}", f"downloads/ops/{ver}/{name}"):
    print(" put", key)
    s3.upload_file(apk, bucket, key, ExtraArgs=extra)
print("r2 ok")
PY

# Mirror into web-dist as a fallback
cp -f "${REMOTE_DIR}/${APK_NAME}" "/opt/techlane/web-dist/web-ops/downloads/${APK_NAME}"

# Register for in-app Update banner
NOTES_SQL="${NOTES//\'/\'\'}"
docker exec -i techlane-unganisha-postgres-1 psql -U techlane -d techlane -v ON_ERROR_STOP=1 <<SQL
INSERT INTO platform.app_releases (
  id, app, platform, version_code, version_name, min_supported_version_code, download_url, notes
) VALUES (
  gen_random_uuid(), 'ops', 'android', ${VERSION_CODE}, '${VERSION_NAME}', 1,
  '${DOWNLOAD_URL}',
  '${NOTES_SQL}'
)
ON CONFLICT (app, platform, version_code) DO UPDATE SET
  version_name = EXCLUDED.version_name,
  download_url = EXCLUDED.download_url,
  notes = EXCLUDED.notes,
  min_supported_version_code = EXCLUDED.min_supported_version_code;
SQL

echo "DOWNLOAD_URL=${DOWNLOAD_URL}"
rm -rf "${REMOTE_DIR}"
REMOTE

# Capture download URL (recompute locally for HTML rewrite)
R2_PUBLIC_BASE="${R2_PUBLIC_BASE:-$R2_PUBLIC_BASE_DEFAULT}"
R2_PUBLIC_BASE="${R2_PUBLIC_BASE%/}"
DOWNLOAD_URL="${R2_PUBLIC_BASE}/downloads/${APK_NAME}?v=${VERSION_NAME}"

echo "==> Updating download pages → ${DOWNLOAD_URL}"
python3 - <<PY
from pathlib import Path
import re
url = "${DOWNLOAD_URL}"
ver = "${VERSION_NAME}"
for path in [
  "apps/web-ops/public/download/index.html",
  "apps/web-portal/public/download/index.html",
  "apps/web-supplier/public/download/index.html",
  "apps/web-storefront/public/download/index.html",
  "design-tokens/brand/download.html",
]:
    p = Path(path)
    if not p.exists():
        continue
    text = p.read_text()
    text = re.sub(r'(com\.techlane\.ops · staff · )v[0-9.]+', rf'\1v{ver}', text)
    text = re.sub(r'href="[^"]*techlane-ops\.apk[^"]*"', f'href="{url}"', text, count=1)
    p.write_text(text)
    print("updated", path)
PY

"${SCP[@]}" apps/web-ops/public/download/index.html \
  "${PUBLISH_SSH}:/opt/techlane/web-dist/web-ops/download/index.html"

echo "==> Smoke-check /app-version"
curl -fsS "https://api.techlane.co.ke/api/v1/app-version?app=ops&platform=android&current_version_code=$((VERSION_CODE - 1))" \
  | python3 -c 'import sys,json; d=json.load(sys.stdin); assert d.get("update_available") is True, d; print("update_available=true for older phones ✓")'

echo
echo "==> Published Ops ${VERSION_NAME} (code ${VERSION_CODE})"
echo "    ${DOWNLOAD_URL}"
echo "    Open Ops on an older install → Update banner → Update"

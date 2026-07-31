-- Mobile app release catalog: lets the Android apps ask "am I outdated?"
-- on launch (see internal/appversion) without needing a Play Store rollout.
CREATE TABLE IF NOT EXISTS platform.app_releases (
  id UUID PRIMARY KEY,
  app TEXT NOT NULL,
  platform TEXT NOT NULL DEFAULT 'android',
  version_code INT NOT NULL,
  version_name TEXT NOT NULL,
  min_supported_version_code INT NOT NULL DEFAULT 1,
  download_url TEXT,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_app_releases_app_platform_version
  ON platform.app_releases (app, platform, version_code);

CREATE INDEX IF NOT EXISTS idx_app_releases_latest
  ON platform.app_releases (app, platform, version_code DESC);

-- Seed the current shipped builds so the endpoint has something to serve
-- from day one; publish new rows via POST /app-releases when cutting a build.
INSERT INTO platform.app_releases (id, app, platform, version_code, version_name, min_supported_version_code, notes)
VALUES
  ('11111111-0000-4000-a000-000000000001', 'ops', 'android', 1, '0.1.0', 1, 'Initial release'),
  ('11111111-0000-4000-a000-000000000002', 'customer', 'android', 1, '0.1.0', 1, 'Initial release'),
  ('11111111-0000-4000-a000-000000000003', 'supplier', 'android', 1, '0.1.0', 1, 'Initial release')
ON CONFLICT (app, platform, version_code) DO NOTHING;

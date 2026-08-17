-- SiteWatch Phase 1 schema + seed data.
-- Auto-executed by the postgres image on first container start (see
-- infra/docker-compose.yml, mounted at /docker-entrypoint-initdb.d/init.sql).
--
-- Notes on deliberate design choices (see docs/PROJECT_PLAN.md / the Phase 1
-- schema review for the full reasoning):
--   - UUID primary keys: ids cross into MongoDB (telemetry_raw.device_id) and
--     into MQTT topic strings, so they must be globally unique on their own.
--   - TEXT + CHECK instead of native ENUM: cheaper to extend later than
--     Postgres enum types.
--   - devices has no mqtt_topic column: the topic is derived from
--     (site_id, id) by application code, not stored, so it can't drift out
--     of sync with the ids it's built from.
--   - alerts.device_id duplicates the device already reachable via
--     alert_rules: the dashboard's most common query is "open alerts for
--     this device", so it gets a direct indexed column instead of a join.
--   - ON DELETE RESTRICT on the operational chain (sites -> devices ->
--     alert_rules -> alerts): deleting something with dependent rows must be
--     an explicit, deliberate action, not an accidental cascade that wipes
--     history.
--   - ON DELETE SET NULL on the two user references on alerts: removing a
--     user account should never be blocked by, or destroy, old alert
--     history.

BEGIN;

CREATE TABLE sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('data_center', 'building', 'factory')),
    location TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('smart_meter', 'temp_sensor', 'humidity_sensor', 'ups', 'hvac')),
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_devices_site_id ON devices (site_id);

CREATE TABLE alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
    metric TEXT NOT NULL,
    operator TEXT NOT NULL CHECK (operator IN ('gt', 'gte', 'lt', 'lte')),
    threshold NUMERIC NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('warning', 'critical')),
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_alert_rules_device_id ON alert_rules (device_id);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'operator', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_rule_id UUID NOT NULL REFERENCES alert_rules(id) ON DELETE RESTRICT,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
    triggered_value NUMERIC NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('warning', 'critical')),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'acknowledged', 'resolved')),
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by UUID REFERENCES users(id) ON DELETE SET NULL,
    resolved_at TIMESTAMPTZ,
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX idx_alerts_device_id ON alerts (device_id);
CREATE INDEX idx_alerts_status ON alerts (status);

-- Seed data -----------------------------------------------------------------
-- Fixed ids so the device simulator, and anyone re-running this file, can
-- reference the same devices without querying for them first.

INSERT INTO sites (id, name, type, location) VALUES
    ('11111111-1111-1111-1111-111111111111', 'Bangkok Data Center', 'data_center', 'Bangkok, TH'),
    ('22222222-2222-2222-2222-222222222222', 'Rayong Factory', 'factory', 'Rayong, TH');

INSERT INTO devices (id, site_id, name, type) VALUES
    ('a1111111-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'smart-meter-01', 'smart_meter'),
    ('a1111111-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111', 'temp-sensor-01', 'temp_sensor'),
    ('a1111111-0000-0000-0000-000000000003', '11111111-1111-1111-1111-111111111111', 'humidity-sensor-01', 'humidity_sensor'),
    ('a1111111-0000-0000-0000-000000000004', '11111111-1111-1111-1111-111111111111', 'ups-01', 'ups'),
    ('a2222222-0000-0000-0000-000000000001', '22222222-2222-2222-2222-222222222222', 'smart-meter-02', 'smart_meter'),
    ('a2222222-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222', 'temp-sensor-02', 'temp_sensor'),
    ('a2222222-0000-0000-0000-000000000003', '22222222-2222-2222-2222-222222222222', 'hvac-01', 'hvac');

INSERT INTO alert_rules (device_id, metric, operator, threshold, severity) VALUES
    ('a1111111-0000-0000-0000-000000000002', 'temperature_c', 'gt', 28, 'warning'),
    ('a1111111-0000-0000-0000-000000000002', 'temperature_c', 'gt', 32, 'critical'),
    ('a1111111-0000-0000-0000-000000000001', 'power_kw', 'gt', 50, 'warning');

-- Fixed id, same reasoning as the sites/devices seed rows above: the
-- Phase 3 frontend has no login flow (no auth exists yet — see
-- docs/PROJECT_PLAN.md), so it references this "acting user" by a known,
-- stable id (VITE_ACTING_USER_ID) instead of discovering it at runtime.
INSERT INTO users (id, email, name, role) VALUES
    ('33333333-0000-0000-0000-000000000001', 'ops@sitewatch.local', 'Site Operator', 'operator');

COMMIT;

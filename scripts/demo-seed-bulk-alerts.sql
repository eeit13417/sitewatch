-- Incident 1 demo only (docs/rca/01-missing-index.md) — NOT part of
-- infra/postgres/init.sql, deliberately: this generates synthetic volume
-- to make a missing index's cost visible with EXPLAIN ANALYZE, not real
-- seed data, and 300k extra rows would slow down every CI run and
-- testcontainers test if it lived in init.sql.
--
-- Run:  PGPASSWORD=sitewatch psql -h localhost -U sitewatch -d sitewatch -f scripts/demo-seed-bulk-alerts.sql
-- Undo: PGPASSWORD=sitewatch psql -h localhost -U sitewatch -d sitewatch -c "DELETE FROM alerts WHERE triggered_value = -1;"
--   (-1 is a value no real alert engine decision ever produces — used here
--   purely as a marker so the synthetic rows are trivially identifiable
--   and removable afterward, without touching real seed/demo alerts.)

INSERT INTO alerts (alert_rule_id, device_id, triggered_value, severity, status, triggered_at)
SELECT
    ar.id,
    ar.device_id,
    -1,                                     -- marker value, see header
    ar.severity,
    'resolved',
    now() - (random() * interval '365 days')
FROM alert_rules ar
CROSS JOIN generate_series(1, 100000) AS g(n);

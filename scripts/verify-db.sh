#!/usr/bin/env bash
# Automates the checks we ran manually while reviewing the Phase 1 schema:
# the five tables exist, the seed data landed, and the constraints from the
# schema review (ON DELETE RESTRICT, the alerts.severity CHECK) actually
# hold. Safe to re-run — the constraint checks roll back or are expected to
# fail, so no data is left behind.
set -euo pipefail

PGHOST="${PGHOST:-localhost}"
PGPORT="${PGPORT:-5432}"
PGUSER="${PGUSER:-sitewatch}"
PGDATABASE="${PGDATABASE:-sitewatch}"
export PGPASSWORD="${PGPASSWORD:-sitewatch}"

run_sql() {
  psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" -tAc "$1"
}

fail() { echo "FAIL: $1" >&2; exit 1; }
pass() { echo "PASS: $1"; }

table_count=$(run_sql "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('sites','devices','alert_rules','alerts','users');")
[ "$table_count" -eq 5 ] || fail "expected 5 tables, found $table_count"
pass "all 5 tables exist"

check_count() {
  local table=$1 expected=$2
  local actual
  actual=$(run_sql "SELECT count(*) FROM $table;")
  [ "$actual" -eq "$expected" ] || fail "$table: expected $expected seed rows, found $actual"
  pass "$table has $expected seed rows"
}
check_count sites 2
check_count devices 7
check_count alert_rules 3
check_count users 1

if run_sql "DELETE FROM sites WHERE name = 'Bangkok Data Center';" >/dev/null 2>&1; then
  fail "ON DELETE RESTRICT did not block deleting a site that still has devices"
fi
pass "ON DELETE RESTRICT blocks deleting a referenced site"

if run_sql "
  BEGIN;
  INSERT INTO alerts (alert_rule_id, device_id, triggered_value, severity)
  SELECT id, device_id, 99, 'not_a_real_severity' FROM alert_rules LIMIT 1;
  ROLLBACK;
" >/dev/null 2>&1; then
  fail "CHECK constraint did not reject an invalid alerts.severity value"
fi
pass "CHECK constraint rejects invalid alerts.severity"

echo "All database checks passed."

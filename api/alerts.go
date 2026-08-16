package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
)

func (a *App) listAlerts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	query := `
		SELECT a.id, a.alert_rule_id, a.device_id, a.triggered_value, a.severity, a.status,
		       a.triggered_at, a.acknowledged_at, a.acknowledged_by, a.resolved_at, a.resolved_by
		FROM alerts a`
	var joins []string
	var conditions []string
	var args []any

	if siteID := q.Get("site_id"); siteID != "" {
		joins = append(joins, "JOIN devices d ON d.id = a.device_id")
		args = append(args, siteID)
		conditions = append(conditions, fmt.Sprintf("d.site_id = $%d", len(args)))
	}
	if status := q.Get("status"); status != "" {
		args = append(args, status)
		conditions = append(conditions, fmt.Sprintf("a.status = $%d", len(args)))
	}
	if deviceID := q.Get("device_id"); deviceID != "" {
		args = append(args, deviceID)
		conditions = append(conditions, fmt.Sprintf("a.device_id = $%d", len(args)))
	}

	for _, j := range joins {
		query += " " + j
	}
	for i, c := range conditions {
		if i == 0 {
			query += " WHERE " + c
		} else {
			query += " AND " + c
		}
	}
	query += " ORDER BY a.triggered_at DESC"

	limit := parseIntParam(q, "limit", 50, 500)
	offset := parseIntParam(q, "offset", 0, 1_000_000)
	args = append(args, limit)
	query += fmt.Sprintf(" LIMIT $%d", len(args))
	args = append(args, offset)
	query += fmt.Sprintf(" OFFSET $%d", len(args))

	rows, err := a.pg.Query(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("query alerts", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list alerts")
		return
	}
	defer rows.Close()

	alerts := []Alert{}
	for rows.Next() {
		var al Alert
		if err := rows.Scan(&al.ID, &al.AlertRuleID, &al.DeviceID, &al.TriggeredValue, &al.Severity,
			&al.Status, &al.TriggeredAt, &al.AcknowledgedAt, &al.AcknowledgedBy, &al.ResolvedAt, &al.ResolvedBy); err != nil {
			a.logger.Error("scan alert", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to list alerts")
			return
		}
		alerts = append(alerts, al)
	}
	writeJSON(w, http.StatusOK, alerts)
}

func (a *App) acknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	a.transitionAlert(w, r, transitionSpec{
		fromStatuses: []string{"open"},
		conflictMsg:  "alert is not open (already acknowledged or resolved)",
		query: `
			UPDATE alerts SET status = 'acknowledged', acknowledged_at = now(), acknowledged_by = $2
			WHERE id = $1 AND status = 'open'
			RETURNING id, alert_rule_id, device_id, triggered_value, severity, status,
			          triggered_at, acknowledged_at, acknowledged_by, resolved_at, resolved_by`,
	})
}

func (a *App) resolveAlert(w http.ResponseWriter, r *http.Request) {
	a.transitionAlert(w, r, transitionSpec{
		fromStatuses: []string{"open", "acknowledged"},
		conflictMsg:  "alert is already resolved",
		query: `
			UPDATE alerts SET status = 'resolved', resolved_at = now(), resolved_by = $2
			WHERE id = $1 AND status IN ('open', 'acknowledged')
			RETURNING id, alert_rule_id, device_id, triggered_value, severity, status,
			          triggered_at, acknowledged_at, acknowledged_by, resolved_at, resolved_by`,
	})
}

type transitionSpec struct {
	fromStatuses []string
	conflictMsg  string
	query        string
}

// transitionAlert runs an UPDATE ... RETURNING that only matches rows in an
// allowed starting status. If it matches nothing, a follow-up lookup
// distinguishes "alert doesn't exist" (404) from "wrong status" (409) —
// the state check itself stays a single atomic statement, so concurrent
// requests can't both succeed against the same alert.
func (a *App) transitionAlert(w http.ResponseWriter, r *http.Request, spec transitionSpec) {
	id := r.PathValue("id")

	var body AckInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	var al Alert
	err := a.pg.QueryRow(r.Context(), spec.query, id, body.UserID).Scan(
		&al.ID, &al.AlertRuleID, &al.DeviceID, &al.TriggeredValue, &al.Severity, &al.Status,
		&al.TriggeredAt, &al.AcknowledgedAt, &al.AcknowledgedBy, &al.ResolvedAt, &al.ResolvedBy)

	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		_ = a.pg.QueryRow(r.Context(), `SELECT true FROM alerts WHERE id = $1`, id).Scan(&exists)
		if !exists {
			writeError(w, http.StatusNotFound, "alert not found")
			return
		}
		writeError(w, http.StatusConflict, spec.conflictMsg)
		return
	}
	if err != nil {
		a.logger.Error("transition alert", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update alert")
		return
	}
	writeJSON(w, http.StatusOK, al)
}

func parseIntParam(q map[string][]string, key string, def, max int) int {
	raw := q[key]
	if len(raw) == 0 || raw[0] == "" {
		return def
	}
	n, err := strconv.Atoi(raw[0])
	if err != nil || n < 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

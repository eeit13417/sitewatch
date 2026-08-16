package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

func (a *App) listAlertRules(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")

	query := `SELECT id, device_id, metric, operator, threshold, severity, enabled FROM alert_rules`
	args := []any{}
	if deviceID != "" {
		query += ` WHERE device_id = $1`
		args = append(args, deviceID)
	}
	query += ` ORDER BY metric`

	rows, err := a.pg.Query(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("query alert_rules", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list alert rules")
		return
	}
	defer rows.Close()

	ruleList := []AlertRule{}
	for rows.Next() {
		var ar AlertRule
		if err := rows.Scan(&ar.ID, &ar.DeviceID, &ar.Metric, &ar.Operator, &ar.Threshold, &ar.Severity, &ar.Enabled); err != nil {
			a.logger.Error("scan alert_rule", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to list alert rules")
			return
		}
		ruleList = append(ruleList, ar)
	}
	writeJSON(w, http.StatusOK, ruleList)
}

func (a *App) createAlertRule(w http.ResponseWriter, r *http.Request) {
	var in AlertRuleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if in.DeviceID == "" || in.Metric == "" || in.Operator == "" || in.Severity == "" {
		writeError(w, http.StatusBadRequest, "device_id, metric, operator, threshold, severity are required")
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}

	var ar AlertRule
	err := a.pg.QueryRow(r.Context(), `
		INSERT INTO alert_rules (device_id, metric, operator, threshold, severity, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, device_id, metric, operator, threshold, severity, enabled`,
		in.DeviceID, in.Metric, in.Operator, in.Threshold, in.Severity, enabled,
	).Scan(&ar.ID, &ar.DeviceID, &ar.Metric, &ar.Operator, &ar.Threshold, &ar.Severity, &ar.Enabled)
	if err != nil {
		a.logger.Error("insert alert_rule", "error", err)
		writeError(w, http.StatusBadRequest, "failed to create alert rule (check device_id, operator, severity)")
		return
	}
	writeJSON(w, http.StatusCreated, ar)
}

func (a *App) updateAlertRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var in AlertRuleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}

	var ar AlertRule
	err := a.pg.QueryRow(r.Context(), `
		UPDATE alert_rules
		SET metric = $2, operator = $3, threshold = $4, severity = $5, enabled = $6
		WHERE id = $1
		RETURNING id, device_id, metric, operator, threshold, severity, enabled`,
		id, in.Metric, in.Operator, in.Threshold, in.Severity, enabled,
	).Scan(&ar.ID, &ar.DeviceID, &ar.Metric, &ar.Operator, &ar.Threshold, &ar.Severity, &ar.Enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "alert rule not found")
		return
	}
	if err != nil {
		a.logger.Error("update alert_rule", "error", err)
		writeError(w, http.StatusBadRequest, "failed to update alert rule")
		return
	}
	writeJSON(w, http.StatusOK, ar)
}

func (a *App) deleteAlertRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	tag, err := a.pg.Exec(r.Context(), `DELETE FROM alert_rules WHERE id = $1`, id)
	if err != nil {
		// Most likely ON DELETE RESTRICT from alerts.alert_rule_id.
		writeError(w, http.StatusConflict, "alert rule still has alerts referencing it")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "alert rule not found")
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

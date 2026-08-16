package main

import (
	"context"
	"fmt"
	"time"
)

type Rule struct {
	ID        string
	Metric    string
	Operator  string
	Threshold float64
	Severity  string
}

type Decision int

const (
	NoAction Decision = iota
	CreateAlert
	AutoResolve
)

type RuleDecision struct {
	Rule     Rule
	Decision Decision
	Value    float64
}

// EvaluateRules is pure — no I/O — so it's unit-tested without a database.
// See docs/alert-engine.md for the decision table this implements.
func EvaluateRules(rules []Rule, readings map[string]float64, activeRuleIDs map[string]bool) []RuleDecision {
	var decisions []RuleDecision
	for _, rule := range rules {
		value, ok := readings[rule.Metric]
		if !ok {
			continue
		}

		triggered := compareThreshold(value, rule.Operator, rule.Threshold)
		active := activeRuleIDs[rule.ID]

		switch {
		case triggered && !active:
			decisions = append(decisions, RuleDecision{Rule: rule, Decision: CreateAlert, Value: value})
		case !triggered && active:
			decisions = append(decisions, RuleDecision{Rule: rule, Decision: AutoResolve, Value: value})
		}
	}
	return decisions
}

func compareThreshold(value float64, operator string, threshold float64) bool {
	switch operator {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	default:
		return false
	}
}

// LoadEnabledRules and LoadActiveRuleIDs feed EvaluateRules; ApplyDecisions
// writes its output. Kept separate from EvaluateRules itself so the
// decision logic stays testable without Postgres.

func (s *Store) LoadEnabledRules(ctx context.Context, deviceID string) ([]Rule, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id, metric, operator, threshold, severity
		FROM alert_rules
		WHERE device_id = $1 AND enabled = true`, deviceID)
	if err != nil {
		return nil, fmt.Errorf("query alert_rules: %w", err)
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.Metric, &r.Operator, &r.Threshold, &r.Severity); err != nil {
			return nil, fmt.Errorf("scan alert_rules: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *Store) LoadActiveRuleIDs(ctx context.Context, deviceID string) (map[string]bool, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT alert_rule_id FROM alerts
		WHERE device_id = $1 AND status IN ('open', 'acknowledged')`, deviceID)
	if err != nil {
		return nil, fmt.Errorf("query active alerts: %w", err)
	}
	defer rows.Close()

	active := make(map[string]bool)
	for rows.Next() {
		var ruleID string
		if err := rows.Scan(&ruleID); err != nil {
			return nil, fmt.Errorf("scan active alerts: %w", err)
		}
		active[ruleID] = true
	}
	return active, rows.Err()
}

// ApplyDecisions writes the outcome of EvaluateRules to Postgres. Relies on
// the alerts schema's own CHECK/FK constraints for correctness; doesn't
// re-validate anything EvaluateRules already guaranteed.
func (s *Store) ApplyDecisions(ctx context.Context, deviceID string, decisions []RuleDecision, at time.Time) error {
	for _, d := range decisions {
		switch d.Decision {
		case CreateAlert:
			_, err := s.pg.Exec(ctx, `
				INSERT INTO alerts (alert_rule_id, device_id, triggered_value, severity, status, triggered_at)
				VALUES ($1, $2, $3, $4, 'open', $5)`,
				d.Rule.ID, deviceID, d.Value, d.Rule.Severity, at)
			if err != nil {
				return fmt.Errorf("insert alert for rule %s: %w", d.Rule.ID, err)
			}
		case AutoResolve:
			_, err := s.pg.Exec(ctx, `
				UPDATE alerts SET status = 'resolved', resolved_at = $1
				WHERE alert_rule_id = $2 AND status IN ('open', 'acknowledged')`,
				at, d.Rule.ID)
			if err != nil {
				return fmt.Errorf("resolve alert for rule %s: %w", d.Rule.ID, err)
			}
		}
	}
	return nil
}

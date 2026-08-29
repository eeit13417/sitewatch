package main

import (
	"context"
	"fmt"
	"sync"
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

// BreachState is one rule's debounce streak: how many consecutive readings
// in a row have gone the same direction (breaching or not) since the
// streak last flipped. See docs/rca/05-alert-storm.md and
// docs/alert-engine.md for why this exists — without it, a value
// oscillating right at a rule's threshold flaps an alert
// create/resolve/create/resolve every single reading.
type BreachState struct {
	Count     int
	Breaching bool
}

// EvaluateRules is pure — no I/O — so it's unit-tested without a database.
// See docs/alert-engine.md for the decision table this implements.
//
// streaks holds each rule's current BreachState (keyed by rule.ID) and is
// mutated in place — the caller owns its lifetime (see BreachTracker below
// for the concurrency-safe holder actually used by ingestion). debounceN is
// how many consecutive same-direction readings are required before a
// decision fires; debounceN=1 reproduces the original no-debounce
// behavior exactly (acts on the very first reading), which is what the
// pre-Phase-6 test cases below still exercise.
func EvaluateRules(rules []Rule, readings map[string]float64, activeRuleIDs map[string]bool, streaks map[string]BreachState, debounceN int) []RuleDecision {
	var decisions []RuleDecision
	for _, rule := range rules {
		value, ok := readings[rule.Metric]
		if !ok {
			continue
		}

		triggered := compareThreshold(value, rule.Operator, rule.Threshold)
		active := activeRuleIDs[rule.ID]

		state := streaks[rule.ID]
		if state.Breaching == triggered {
			state.Count++
		} else {
			state = BreachState{Breaching: triggered, Count: 1}
		}
		streaks[rule.ID] = state

		switch {
		case triggered && !active && state.Count >= debounceN:
			decisions = append(decisions, RuleDecision{Rule: rule, Decision: CreateAlert, Value: value})
		case !triggered && active && state.Count >= debounceN:
			decisions = append(decisions, RuleDecision{Rule: rule, Decision: AutoResolve, Value: value})
		}
	}
	return decisions
}

// BreachTracker is the concurrency-safe holder for debounce state across
// messages: ingestion's worker pool (main.go) processes multiple devices'
// messages concurrently, and different devices' rules could in principle
// still land in the same map, so all access needs the mutex (CLAUDE.md
// rule 7) — real state to encapsulate, unlike the pure EvaluateRules
// function it wraps (rule 1).
type BreachTracker struct {
	mu      sync.Mutex
	streaks map[string]BreachState
}

func NewBreachTracker() *BreachTracker {
	return &BreachTracker{streaks: make(map[string]BreachState)}
}

func (t *BreachTracker) Evaluate(rules []Rule, readings map[string]float64, activeRuleIDs map[string]bool, debounceN int) []RuleDecision {
	t.mu.Lock()
	defer t.mu.Unlock()
	return EvaluateRules(rules, readings, activeRuleIDs, t.streaks, debounceN)
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

// applyDecisionsWithRetry retries a transient Postgres failure writing
// alert decisions a bounded number of times before giving up — see
// docs/rca/04-mongo-postgres-inconsistency.md. By the time this runs,
// telemetry has already committed to MongoDB (process() in main.go writes
// it first), so losing this write entirely — not just delaying it — means
// a condition that should have alerted silently never does, with nothing
// to reconcile it later. A short bounded retry closes the gap for the
// realistic failure mode (a brief connection blip); a full outbox/
// reconciliation system would be needed to survive a sustained outage and
// is a deliberately deferred bigger investment — this project runs as a
// single ingestion instance with no durability requirement beyond "don't
// drop an alert over a hiccup."
func applyDecisionsWithRetry(ctx context.Context, store *Store, deviceID string, decisions []RuleDecision, at time.Time) error {
	const maxAttempts = 3
	backoff := 100 * time.Millisecond

	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err = store.ApplyDecisions(ctx, deviceID, decisions, at); err == nil {
			return nil
		}
		if attempt < maxAttempts {
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	return fmt.Errorf("apply decisions after %d attempts: %w", maxAttempts, err)
}

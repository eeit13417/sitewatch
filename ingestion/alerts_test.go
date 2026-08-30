package main

import (
	"reflect"
	"testing"
)

// debounceN=1 reproduces the original no-debounce behavior exactly (acts
// on the very first reading) — every test in this file that isn't
// specifically exercising debounce uses it so the pre-Phase-6 cases still
// verify the same thing they always did.
const noDebounce = 1

func TestEvaluateRules_CreatesAlertWhenTriggeredAndNotActive(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "temperature_c", Operator: "gt", Threshold: 30, Severity: "warning"}}
	readings := map[string]float64{"temperature_c": 33}

	got := EvaluateRules(rules, readings, map[string]bool{}, map[string]BreachState{}, noDebounce)

	want := []RuleDecision{{Rule: rules[0], Decision: CreateAlert, Value: 33}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestEvaluateRules_NoActionWhenTriggeredAndAlreadyActive(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "temperature_c", Operator: "gt", Threshold: 30, Severity: "warning"}}
	readings := map[string]float64{"temperature_c": 33}
	active := map[string]bool{"r1": true}

	got := EvaluateRules(rules, readings, active, map[string]BreachState{}, noDebounce)

	if len(got) != 0 {
		t.Fatalf("expected no decisions (already active), got %+v", got)
	}
}

func TestEvaluateRules_AutoResolvesWhenNoLongerTriggeredAndActive(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "temperature_c", Operator: "gt", Threshold: 30, Severity: "warning"}}
	readings := map[string]float64{"temperature_c": 24}
	active := map[string]bool{"r1": true}

	got := EvaluateRules(rules, readings, active, map[string]BreachState{}, noDebounce)

	want := []RuleDecision{{Rule: rules[0], Decision: AutoResolve, Value: 24}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestEvaluateRules_NoActionWhenNotTriggeredAndNotActive(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "temperature_c", Operator: "gt", Threshold: 30, Severity: "warning"}}
	readings := map[string]float64{"temperature_c": 24}

	got := EvaluateRules(rules, readings, map[string]bool{}, map[string]BreachState{}, noDebounce)

	if len(got) != 0 {
		t.Fatalf("expected no decisions, got %+v", got)
	}
}

func TestEvaluateRules_IgnoresRuleWhoseMetricIsMissingFromReadings(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "power_kw", Operator: "gt", Threshold: 50, Severity: "warning"}}
	readings := map[string]float64{"temperature_c": 33} // power_kw absent

	got := EvaluateRules(rules, readings, map[string]bool{}, map[string]BreachState{}, noDebounce)

	if len(got) != 0 {
		t.Fatalf("expected no decisions for a metric the device didn't report, got %+v", got)
	}
}

func TestEvaluateRules_EvaluatesEachRuleIndependently(t *testing.T) {
	rules := []Rule{
		{ID: "warn", Metric: "temperature_c", Operator: "gt", Threshold: 28, Severity: "warning"},
		{ID: "crit", Metric: "temperature_c", Operator: "gt", Threshold: 32, Severity: "critical"},
	}
	readings := map[string]float64{"temperature_c": 30} // breaches warning, not critical
	active := map[string]bool{"crit": true}             // critical was previously active

	got := EvaluateRules(rules, readings, active, map[string]BreachState{}, noDebounce)

	want := []RuleDecision{
		{Rule: rules[0], Decision: CreateAlert, Value: 30}, // warning newly triggers
		{Rule: rules[1], Decision: AutoResolve, Value: 30}, // critical no longer holds
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestCompareThreshold(t *testing.T) {
	cases := []struct {
		operator string
		value    float64
		want     bool
	}{
		{"gt", 5, false}, {"gt", 5.1, true},
		{"gte", 5, true}, {"gte", 4.9, false},
		{"lt", 5, false}, {"lt", 4.9, true},
		{"lte", 5, true}, {"lte", 5.1, false},
		{"unknown", 999, false},
	}
	for _, c := range cases {
		if got := compareThreshold(c.value, c.operator, 5); got != c.want {
			t.Errorf("compareThreshold(%v, %q, 5) = %v, want %v", c.value, c.operator, got, c.want)
		}
	}
}

// --- Debounce (Phase 6, incident 5) -----------------------------------

func TestEvaluateRules_Debounce_WithholdsCreateUntilNConsecutiveBreaches(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "temperature_c", Operator: "gt", Threshold: 30, Severity: "warning"}}
	readings := map[string]float64{"temperature_c": 33}
	streaks := map[string]BreachState{}
	const debounceN = 3

	for i := 1; i < debounceN; i++ {
		got := EvaluateRules(rules, readings, map[string]bool{}, streaks, debounceN)
		if len(got) != 0 {
			t.Fatalf("breach %d/%d: expected no decision yet, got %+v", i, debounceN, got)
		}
	}

	got := EvaluateRules(rules, readings, map[string]bool{}, streaks, debounceN)
	want := []RuleDecision{{Rule: rules[0], Decision: CreateAlert, Value: 33}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("on the Nth consecutive breach: got %+v, want %+v", got, want)
	}
}

func TestEvaluateRules_Debounce_ResetsStreakWhenDirectionFlips(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "temperature_c", Operator: "gt", Threshold: 30, Severity: "warning"}}
	streaks := map[string]BreachState{}
	const debounceN = 3

	// Two breaches, then it dips back under threshold — streak should reset,
	// not carry over toward the next breach.
	EvaluateRules(rules, map[string]float64{"temperature_c": 33}, map[string]bool{}, streaks, debounceN)
	EvaluateRules(rules, map[string]float64{"temperature_c": 33}, map[string]bool{}, streaks, debounceN)
	EvaluateRules(rules, map[string]float64{"temperature_c": 24}, map[string]bool{}, streaks, debounceN)

	got := EvaluateRules(rules, map[string]float64{"temperature_c": 33}, map[string]bool{}, streaks, debounceN)
	if len(got) != 0 {
		t.Fatalf("expected the interrupted streak to have reset (this is only breach 1/3 again), got %+v", got)
	}
}

func TestEvaluateRules_Debounce_WithholdsAutoResolveUntilNConsecutiveNonBreaches(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "temperature_c", Operator: "gt", Threshold: 30, Severity: "warning"}}
	active := map[string]bool{"r1": true}
	streaks := map[string]BreachState{}
	const debounceN = 3

	for i := 1; i < debounceN; i++ {
		got := EvaluateRules(rules, map[string]float64{"temperature_c": 24}, active, streaks, debounceN)
		if len(got) != 0 {
			t.Fatalf("non-breach %d/%d: expected no decision yet, got %+v", i, debounceN, got)
		}
	}

	got := EvaluateRules(rules, map[string]float64{"temperature_c": 24}, active, streaks, debounceN)
	want := []RuleDecision{{Rule: rules[0], Decision: AutoResolve, Value: 24}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("on the Nth consecutive non-breach: got %+v, want %+v", got, want)
	}
}

// TestEvaluateRules_Debounce_StopsFlappingAtTheThreshold is the actual
// incident-5 scenario: a value oscillating right at the boundary. Without
// debounce this creates/resolves an alert on every single reading;
// EvaluateRules never sees 3 consecutive readings in the same direction,
// so it should never act at all.
func TestEvaluateRules_Debounce_StopsFlappingAtTheThreshold(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "temperature_c", Operator: "gt", Threshold: 30, Severity: "warning"}}
	streaks := map[string]BreachState{}
	active := map[string]bool{}
	const debounceN = 3

	oscillating := []float64{31, 29, 31, 29, 31, 29, 31, 29}
	for _, v := range oscillating {
		got := EvaluateRules(rules, map[string]float64{"temperature_c": v}, active, streaks, debounceN)
		if len(got) != 0 {
			t.Fatalf("value %v: expected debounce to suppress flapping, got %+v", v, got)
		}
	}
}

package main

import (
	"reflect"
	"testing"
)

func TestEvaluateRules_CreatesAlertWhenTriggeredAndNotActive(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "temperature_c", Operator: "gt", Threshold: 30, Severity: "warning"}}
	readings := map[string]float64{"temperature_c": 33}

	got := EvaluateRules(rules, readings, map[string]bool{})

	want := []RuleDecision{{Rule: rules[0], Decision: CreateAlert, Value: 33}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestEvaluateRules_NoActionWhenTriggeredAndAlreadyActive(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "temperature_c", Operator: "gt", Threshold: 30, Severity: "warning"}}
	readings := map[string]float64{"temperature_c": 33}
	active := map[string]bool{"r1": true}

	got := EvaluateRules(rules, readings, active)

	if len(got) != 0 {
		t.Fatalf("expected no decisions (already active), got %+v", got)
	}
}

func TestEvaluateRules_AutoResolvesWhenNoLongerTriggeredAndActive(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "temperature_c", Operator: "gt", Threshold: 30, Severity: "warning"}}
	readings := map[string]float64{"temperature_c": 24}
	active := map[string]bool{"r1": true}

	got := EvaluateRules(rules, readings, active)

	want := []RuleDecision{{Rule: rules[0], Decision: AutoResolve, Value: 24}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestEvaluateRules_NoActionWhenNotTriggeredAndNotActive(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "temperature_c", Operator: "gt", Threshold: 30, Severity: "warning"}}
	readings := map[string]float64{"temperature_c": 24}

	got := EvaluateRules(rules, readings, map[string]bool{})

	if len(got) != 0 {
		t.Fatalf("expected no decisions, got %+v", got)
	}
}

func TestEvaluateRules_IgnoresRuleWhoseMetricIsMissingFromReadings(t *testing.T) {
	rules := []Rule{{ID: "r1", Metric: "power_kw", Operator: "gt", Threshold: 50, Severity: "warning"}}
	readings := map[string]float64{"temperature_c": 33} // power_kw absent

	got := EvaluateRules(rules, readings, map[string]bool{})

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

	got := EvaluateRules(rules, readings, active)

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

package main

import "time"

type Site struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location string `json:"location,omitempty"`
}

type Device struct {
	ID         string     `json:"id"`
	SiteID     string     `json:"site_id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

type TelemetryPoint struct {
	DeviceID   string             `json:"device_id"`
	Readings   map[string]float64 `json:"readings"`
	Ts         time.Time          `json:"ts"`
	ReceivedAt time.Time          `json:"received_at"`
}

type AlertRule struct {
	ID        string  `json:"id"`
	DeviceID  string  `json:"device_id"`
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
	Severity  string  `json:"severity"`
	Enabled   bool    `json:"enabled"`
}

type AlertRuleInput struct {
	DeviceID  string  `json:"device_id"`
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
	Severity  string  `json:"severity"`
	Enabled   *bool   `json:"enabled"`
}

type Alert struct {
	ID             string     `json:"id"`
	AlertRuleID    string     `json:"alert_rule_id"`
	DeviceID       string     `json:"device_id"`
	TriggeredValue float64    `json:"triggered_value"`
	Severity       string     `json:"severity"`
	Status         string     `json:"status"`
	TriggeredAt    time.Time  `json:"triggered_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	AcknowledgedBy *string    `json:"acknowledged_by"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	ResolvedBy     *string    `json:"resolved_by"`
}

type AckInput struct {
	UserID string `json:"user_id"`
}

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TelemetryMessage mirrors the MQTT payload contract in docs/mqtt-contract.md.
type TelemetryMessage struct {
	DeviceID   string             `json:"device_id"`
	SiteID     string             `json:"site_id"`
	DeviceType string             `json:"device_type"`
	Readings   map[string]float64 `json:"readings"`
	Ts         time.Time          `json:"ts"`
}

func parseTelemetryMessage(raw []byte) (TelemetryMessage, error) {
	var msg TelemetryMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return TelemetryMessage{}, fmt.Errorf("invalid json: %w", err)
	}
	if msg.DeviceID == "" || msg.SiteID == "" || msg.DeviceType == "" {
		return TelemetryMessage{}, fmt.Errorf("missing required field(s)")
	}
	if len(msg.Readings) == 0 {
		return TelemetryMessage{}, fmt.Errorf("readings must not be empty")
	}
	return msg, nil
}

// parseTopic extracts (siteID, deviceID) from a
// "sitewatch/<site_id>/<device_id>/telemetry" topic — the receiving side
// of the same convention simulator/src/mqtt-topic.ts builds.
func parseTopic(topic string) (siteID, deviceID string, err error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 4 || parts[0] != "sitewatch" || parts[3] != "telemetry" {
		return "", "", fmt.Errorf("topic %q does not match sitewatch/<site_id>/<device_id>/telemetry", topic)
	}
	return parts[1], parts[2], nil
}

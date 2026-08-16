package main

import (
	"net/http"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (a *App) listDevices(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")

	query := `SELECT id, site_id, name, type, last_seen_at FROM devices`
	args := []any{}
	if siteID != "" {
		query += ` WHERE site_id = $1`
		args = append(args, siteID)
	}
	query += ` ORDER BY name`

	rows, err := a.pg.Query(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("query devices", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}
	defer rows.Close()

	devices := []Device{}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.SiteID, &d.Name, &d.Type, &d.LastSeenAt); err != nil {
			a.logger.Error("scan device", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to list devices")
			return
		}
		devices = append(devices, d)
	}
	writeJSON(w, http.StatusOK, devices)
}

func (a *App) getDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var d Device
	err := a.pg.QueryRow(r.Context(),
		`SELECT id, site_id, name, type, last_seen_at FROM devices WHERE id = $1`, id,
	).Scan(&d.ID, &d.SiteID, &d.Name, &d.Type, &d.LastSeenAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// getDeviceTelemetry is the one read path that hits MongoDB instead of
// Postgres — raw telemetry lives in telemetry_raw per docs/mqtt-contract.md.
func (a *App) getDeviceTelemetry(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	limit := int64(100)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	filter := bson.M{"device_id": deviceID}
	tsFilter := bson.M{}
	if from := parseTimeParam(r, "from"); from != nil {
		tsFilter["$gte"] = *from
	}
	if to := parseTimeParam(r, "to"); to != nil {
		tsFilter["$lte"] = *to
	}
	if len(tsFilter) > 0 {
		filter["ts"] = tsFilter
	}

	coll := a.mongo.Database(a.mongoDB).Collection("telemetry_raw")
	// Fetch most-recent-first so `limit` caps to the latest readings, then
	// reverse below so the response reads chronologically for charting.
	cursor, err := coll.Find(r.Context(), filter,
		options.Find().SetSort(bson.D{{Key: "ts", Value: -1}}).SetLimit(limit))
	if err != nil {
		a.logger.Error("query telemetry_raw", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to query telemetry")
		return
	}
	defer cursor.Close(r.Context())

	points := []TelemetryPoint{}
	for cursor.Next(r.Context()) {
		var p TelemetryPoint
		if err := cursor.Decode(&p); err != nil {
			a.logger.Error("decode telemetry_raw", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to query telemetry")
			return
		}
		points = append(points, p)
	}
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
	writeJSON(w, http.StatusOK, points)
}

func parseTimeParam(r *http.Request, key string) *time.Time {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &t
}

package main

import "net/http"

func (a *App) listSites(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pg.Query(r.Context(), `SELECT id, name, type, COALESCE(location, '') FROM sites ORDER BY name`)
	if err != nil {
		a.logger.Error("query sites", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list sites")
		return
	}
	defer rows.Close()

	sites := []Site{}
	for rows.Next() {
		var s Site
		if err := rows.Scan(&s.ID, &s.Name, &s.Type, &s.Location); err != nil {
			a.logger.Error("scan site", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to list sites")
			return
		}
		sites = append(sites, s)
	}
	writeJSON(w, http.StatusOK, sites)
}

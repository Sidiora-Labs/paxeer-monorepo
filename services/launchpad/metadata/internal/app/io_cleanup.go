package app

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func writeJSON(w http.ResponseWriter, value any) {
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("metadata response write", "error", err)
	}
}

package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func CheckMyWork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.ContentLength == 0 {
		http.Error(w, "Request body is empty", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	var csvData [][]string
	err := json.NewDecoder(r.Body).Decode(&csvData)
	if err != nil {
		http.Error(w, "Error parsing CSV", http.StatusBadRequest)
		return
	}

	if len(csvData) < 2 {
		http.Error(w, "No rows in CSV to process", http.StatusBadRequest)
	}

	header := csvData[0]

	for rowIndex, row := range csvData[1:] {
		item := make(map[string]string, len(header))
		for colIndex, cell := range row {
			column := header[colIndex]
			item[column] = cell
		}
		slog.Info("Parsed row", "row", rowIndex, "values", item)
	}

	response := map[string]string{
		"A2":  "Failed date format",
		"C12": "File does not exist",
	}
	w.Header().Set("Content-Type", "application/json")
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		slog.Error("Error creating JSON response", "err", err)
		http.Error(w, "Error creating JSON response", http.StatusInternalServerError)
		return
	}

	_, err = w.Write(jsonResponse)
	if err != nil {
		slog.Error("Error writing JSON response", "err", err)
		http.Error(w, "Error writing JSON response", http.StatusInternalServerError)
	}
}

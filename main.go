package main

import (
	"log/slog"
	"net/http"

	"github.com/lehigh-university-libraries/fabricator/internal/handlers"
)

func main() {
	http.HandleFunc("/workbench/check", handlers.CheckMyWork)
	http.HandleFunc("/workbench/transform", handlers.TransformCsv)

	slog.Info("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

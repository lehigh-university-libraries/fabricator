package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/lehigh-university-libraries/fabricator/internal/handlers"
)

func main() {
	http.HandleFunc("/workbench/check", handlers.CheckMyWork)
	http.HandleFunc("/workbench/transform", handlers.TransformCsv)
	http.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	slog.Info("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

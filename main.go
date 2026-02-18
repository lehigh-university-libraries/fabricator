package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/lehigh-university-libraries/fabricator/internal/handlers"
)

func main() {
	name := flag.String("name", "", "Person name for CLI term resolution test")
	institution := flag.String("institution", "", "Institution name for CLI term resolution test")
	orcid := flag.String("orcid", "", "ORCiD value for CLI term resolution test")
	email := flag.String("email", "", "Email value for CLI term resolution test")
	flag.Parse()

	if *name != "" || *institution != "" || *orcid != "" || *email != "" {
		if *name == "" {
			slog.Error("name is required when using CLI term resolution flags")
			os.Exit(1)
		}
		tid, err := handlers.ResolvePersonTermID(*name, *institution, *orcid, *email)
		if err != nil {
			slog.Error("failed resolving person term", "err", err)
			os.Exit(1)
		}
		fmt.Println(tid)
		return
	}

	if os.Getenv("ISLE_SITE_URL") == "" {
		os.Setenv("ISLE_SITE_URL", "https://preserve.lehigh.edu")
	}
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

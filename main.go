package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/lehigh-university-libraries/fabricator/internal/handlers"
)

func main() {
	name := flag.String("name", "", "Person name for CLI term resolution test")
	institution := flag.String("institution", "", "Institution name for CLI term resolution test")
	orcid := flag.String("orcid", "", "ORCiD value for CLI term resolution test")
	email := flag.String("email", "", "Email value for CLI term resolution test")
	checkCSV := flag.String("check-csv", "", "Path to CSV file to run through check (prints JSON result)")
	transformCSV := flag.String("transform-csv", "", "Path to CSV file to run through transform (writes ZIP output)")
	transformOut := flag.String("transform-out", "", "Output path for transform ZIP (default: <input>.zip)")
	flag.Parse()

	if *checkCSV != "" || *transformCSV != "" {
		if *checkCSV != "" {
			if err := runCheckCSV(*checkCSV); err != nil {
				slog.Error("check-csv failed", "err", err)
				os.Exit(1)
			}
		}
		if *transformCSV != "" {
			out := *transformOut
			if out == "" {
				out = fmt.Sprintf("%s.zip", strings.TrimSuffix(*transformCSV, filepath.Ext(*transformCSV)))
			}
			if err := runTransformCSV(*transformCSV, out); err != nil {
				slog.Error("transform-csv failed", "err", err)
				os.Exit(1)
			}
			fmt.Println(out)
		}
		return
	}

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

func runCheckCSV(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return err
	}

	body, err := json.Marshal(rows)
	if err != nil {
		return err
	}

	req := httptest.NewRequest(http.MethodPost, "/workbench/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if os.Getenv("SHARED_SECRET") != "" {
		req.Header.Set("X-Secret", os.Getenv("SHARED_SECRET"))
	}
	rec := httptest.NewRecorder()
	handlers.CheckMyWork(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("check returned %d: %s", resp.StatusCode, string(raw))
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err == nil {
		fmt.Println(pretty.String())
		return nil
	}
	fmt.Println(string(raw))
	return nil
}

func runTransformCSV(path, out string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	req := httptest.NewRequest(http.MethodPost, "/workbench/transform", file)
	req.Header.Set("Content-Type", "text/csv")
	if info, err := os.Stat(path); err == nil {
		req.ContentLength = info.Size()
	}

	rec := httptest.NewRecorder()
	handlers.TransformCsv(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("transform returned %d: %s", resp.StatusCode, string(raw))
	}
	return os.WriteFile(out, raw, 0644)
}

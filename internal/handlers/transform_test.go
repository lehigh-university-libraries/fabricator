package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/lehigh-university-libraries/fabricator/internal/contributor"
)

func TestReadCSVWithJSONTags(t *testing.T) {
	tests := []struct {
		name            string
		csvContent      string
		expectedHeaders []string
		expectedRows    []map[string][]string
		expectError     bool
	}{
		{
			name: "Valid create CSV with headers and rows",
			csvContent: `Title,Object Model,Full Title
foo,bar,Full Test Title`,
			expectedHeaders: []string{
				"title",
				"field_model",
				"field_full_title",
			},
			expectedRows: []map[string][]string{
				{
					"title":            {"foo"},
					"field_model":      {"bar"},
					"field_full_title": {"Full Test Title"},
				},
			},
			expectError: false,
		},
		{
			name: "Valid update CSV with headers and rows",
			csvContent: `Full Title,Node ID
Full Test Title,123`,
			expectedHeaders: []string{
				"node_id",
				"field_full_title",
			},
			expectedRows: []map[string][]string{
				{
					"field_full_title": {"Full Test Title"},
					"node_id":          {"123"},
				},
			},
			expectError: false,
		},
		{
			name: "Valid create CSV with 5-digit upload ID",
			csvContent: `Upload ID,Title,Object Model,Full Title
10000,foo,bar,Full Test Title`,
			expectedHeaders: []string{
				"id",
				"title",
				"field_model",
				"field_full_title",
			},
			expectedRows: []map[string][]string{
				{
					"id":               {"10000"},
					"title":            {"foo"},
					"field_model":      {"bar"},
					"field_full_title": {"Full Test Title"},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate HTTP request with CSV body
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.csvContent))
			req.Header.Set("Content-Type", "text/csv")

			// Call function under test
			headers, rows, err := readCSVWithJSONTags(req)
			firstRow := make([]string, 0, len(headers))
			for header := range headers {
				firstRow = append(firstRow, header)
			}

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Assert headers
			if !equalStringSlices(firstRow, tt.expectedHeaders) {
				t.Errorf("Expected headers %v, got %v", tt.expectedHeaders, firstRow)
			}

			// Assert rows
			if !equalRowSlices(rows, tt.expectedRows) {
				t.Errorf("Expected rows %v, got %v", tt.expectedRows, rows)
			}
		})
	}
}

func equalRowSlices(a, b []map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, rowA := range a {
		rowB := b[i]
		if !equalRow(rowA, rowB) {
			return false
		}
	}
	return true
}

func equalRow(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, vA := range a {
		vB, exists := b[k]
		if !exists || !equalStringSlices(vA, vB) {
			return false
		}
	}
	return true
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strInSlice(a[i], b) {
			return false
		}
	}
	return true
}

func TestResolveContributorEmailLookupPrecedence(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/term_from_term_name" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("email") != "person@example.edu" {
			t.Fatalf("expected email lookup, got query: %s", q.Encode())
		}
		if q.Get("name") != "Smith, Sam" {
			t.Fatalf("expected name lookup when email is present, got query: %s", q.Encode())
		}
		if q.Get("orcid") != "" {
			t.Fatalf("did not expect orcid lookup when email is present, got query: %s", q.Encode())
		}
		if q.Get("works_for") != "" {
			t.Fatalf("did not expect works_for lookup when email is present, got query: %s", q.Encode())
		}
		_, _ = w.Write([]byte(`[{"tid":[{"value":12345}]}]`))
	}))
	defer ts.Close()

	resolver := &drupalTermResolver{
		baseURL:      ts.URL,
		username:     "workbench",
		password:     "secret",
		client:       ts.Client(),
		peopleCache:  map[string]int{},
		institutions: map[string]int{},
	}
	got, err := resolver.resolveContributor(contributor.Contributor{
		Name:        "relators:cre:person:Smith, Sam",
		Email:       "person@example.edu",
		Orcid:       "0000-0000-0000-0000",
		Institution: "Lehigh University",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "relators:cre:12345" {
		t.Fatalf("unexpected resolved contributor: %s", got)
	}
}

func TestResolveContributorNameInstitutionCreate(t *testing.T) {
	var sawPersonCreate bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/term_from_term_name":
			q := r.URL.Query()
			switch q.Get("vocab") {
			case "corporate_body":
				if q.Get("name") != "Lehigh University" {
					t.Fatalf("unexpected institution query: %s", q.Encode())
				}
				_, _ = w.Write([]byte(`[{"tid":[{"value":62}]}]`))
				return
			case "person":
				if q.Get("name") != "Smith, Sam" || q.Get("works_for") != "62" {
					t.Fatalf("expected person lookup by name+institution, got query: %s", q.Encode())
				}
				_, _ = w.Write([]byte(`[]`))
				return
			default:
				t.Fatalf("unexpected vocab in query: %s", q.Encode())
			}
		case "/taxonomy/term":
			if r.URL.RawQuery != "_format=json" {
				t.Fatalf("unexpected taxonomy create query: %s", r.URL.RawQuery)
			}
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("failed decoding payload: %v", err)
			}
			rel, ok := payload["field_relationships"].([]interface{})
			if !ok || len(rel) != 1 {
				t.Fatalf("expected one field_relationships entry, got %#v", payload["field_relationships"])
			}
			relMap, ok := rel[0].(map[string]interface{})
			if !ok {
				t.Fatalf("expected relationship map, got %#v", rel[0])
			}
			if relMap["target_id"] != float64(62) {
				t.Fatalf("expected target_id 62, got %#v", relMap["target_id"])
			}
			sawPersonCreate = true
			_, _ = w.Write([]byte(`{"tid":[{"value":128900}]}`))
			return
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	base, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("failed to parse test URL: %v", err)
	}
	resolver := &drupalTermResolver{
		baseURL:      strings.TrimRight(base.String(), "/"),
		username:     "workbench",
		password:     "secret",
		client:       ts.Client(),
		peopleCache:  map[string]int{},
		institutions: map[string]int{},
	}

	got, err := resolver.resolveContributor(contributor.Contributor{
		Name:        "relators:cre:person:Smith, Sam",
		Institution: "Lehigh University",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sawPersonCreate {
		t.Fatal("expected person create call")
	}
	if got != "relators:cre:128900" {
		t.Fatalf("unexpected resolved contributor: %s", got)
	}
}

func TestResolvePersonTermID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/term_from_term_name" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("name") != "Smith, Sam" || q.Get("vocab") != "person" || q.Get("email") != "person@example.edu" {
			t.Fatalf("unexpected query params: %s", q.Encode())
		}
		_, _ = w.Write([]byte(`[{"tid":[{"value":999}]}]`))
	}))
	defer ts.Close()

	original := os.Getenv("FABRICATOR_TERM_LOOKUP_URL")
	if err := os.Setenv("FABRICATOR_TERM_LOOKUP_URL", ts.URL); err != nil {
		t.Fatalf("failed setting env: %v", err)
	}
	defer func() {
		_ = os.Setenv("FABRICATOR_TERM_LOOKUP_URL", original)
	}()

	tid, err := ResolvePersonTermID("Smith, Sam", "Lehigh University", "0000-0000-0000-0000", "person@example.edu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tid != 999 {
		t.Fatalf("expected tid 999, got %d", tid)
	}
}

func TestResolveContributorUniqueIDNameMismatchCreatesChild(t *testing.T) {
	var sawCreate bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/term_from_term_name":
			q := r.URL.Query()
			if q.Get("vocab") != "person" || q.Get("email") != "person@example.edu" {
				t.Fatalf("unexpected lookup query: %s", q.Encode())
			}
			_, _ = w.Write([]byte(`[{"tid":[{"value":321}],"name":[{"value":"Smith, Sam - Lehigh University"}]}]`))
		case "/taxonomy/term":
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("failed decoding payload: %v", err)
			}

			if _, ok := payload["parent"]; ok {
				t.Fatalf("did not expect parent payload, got %#v", payload["parent"])
			}
			sawCreate = true
			_, _ = w.Write([]byte(`{"tid":[{"value":777}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	resolver := &drupalTermResolver{
		baseURL:      ts.URL,
		username:     "workbench",
		password:     "secret",
		client:       ts.Client(),
		peopleCache:  map[string]int{},
		institutions: map[string]int{},
	}

	got, err := resolver.resolveContributor(contributor.Contributor{
		Name:  "relators:cre:person:Smith, Sam",
		Email: "person@example.edu",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sawCreate {
		t.Fatal("expected create call for name mismatch on unique-id lookup")
	}
	if got != "relators:cre:777" {
		t.Fatalf("unexpected resolved contributor: %s", got)
	}
}

func TestReadCSVWithContributorMapsToFieldLinkedAgent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/term_from_term_name" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("vocab") != "corporate_body" || q.Get("name") != "The Valley Voice" {
			t.Fatalf("unexpected query: %s", q.Encode())
		}
		_, _ = w.Write([]byte(`[{"tid":[{"value":155683}]}]`))
	}))
	defer ts.Close()

	original := os.Getenv("FABRICATOR_TERM_LOOKUP_URL")
	if err := os.Setenv("FABRICATOR_TERM_LOOKUP_URL", ts.URL); err != nil {
		t.Fatalf("failed setting env: %v", err)
	}
	defer func() {
		_ = os.Setenv("FABRICATOR_TERM_LOOKUP_URL", original)
	}()

	csvContent := `Contributor
"{""name"":""relators:cre:corporate_body:The Valley Voice"",""email"":"""",""orcid"":"""",""institution"":"""",""status"":""""}"`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(csvContent))
	req.Header.Set("Content-Type", "text/csv")

	headers, rows, err := readCSVWithJSONTags(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !headers["field_linked_agent"] {
		t.Fatalf("expected field_linked_agent in headers, got %#v", headers)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	got := rows[0]["field_linked_agent"]
	if len(got) != 1 || got[0] != "relators:cre:155683" {
		t.Fatalf("unexpected field_linked_agent value: %#v", got)
	}
}

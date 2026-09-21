package tgn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGetLocationFromTGN(t *testing.T) {
	// Every parent URI points back to this server so hierarchy lookups stay local.
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	data, err := os.ReadFile("testdata/places.json")
	if err != nil {
		t.Fatal(err)
	}
	var responses map[string]json.RawMessage
	if err := json.Unmarshal(data, &responses); err != nil {
		t.Fatal(err)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("expected Accept application/json, got %q", got)
		}
		body, ok := responses[r.URL.Path]
		if !ok {
			t.Errorf("unexpected request path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprint(w, strings.ReplaceAll(string(body), "http://vocab.getty.edu/tgn/", server.URL+"/tgn/")); err != nil {
			t.Errorf("writing response: %v", err)
		}
	})

	tests := map[string]struct {
		URI      string
		Expected Location
	}{
		"Test Bethlehem": {
			URI: server.URL + "/page/tgn/7013416",
			Expected: Location{
				Country:     "United States",
				State:       "Pennsylvania",
				County:      "Northampton",
				City:        "Bethlehem",
				Coordinates: "40.6167,-75.35",
			},
		},
		"Test Coplay": {
			URI: server.URL + "/page/tgn/2087483",
			Expected: Location{
				Country:     "United States",
				State:       "Pennsylvania",
				County:      "Lehigh",
				City:        "Coplay",
				Coordinates: "40.6667,-75.4833",
			},
		},
		"Test Luxembourg": {
			URI: server.URL + "/page/tgn/7003514",
			// This nation has Europe and World above it, not a state or county.
			Expected: Location{
				Country:     "Luxembourg",
				Coordinates: "49.75,6.1667",
			},
		},
		"Test Northampton county": {
			URI: server.URL + "/page/tgn/1002729",
			Expected: Location{
				Country:     "United States",
				State:       "Pennsylvania",
				County:      "Northampton",
				Coordinates: "40.8667,-75.25",
			},
		},
		"Test Pennsylvania state": {
			URI: server.URL + "/page/tgn/7007710",
			Expected: Location{
				Country:     "United States",
				State:       "Pennsylvania",
				Coordinates: "40.8333,-0.6",
			},
		},
		"Test Europe continent": {
			URI: server.URL + "/page/tgn/1000003",
			Expected: Location{
				Coordinates: "56.2,15.016",
			},
		},
		"Test World": {
			URI: server.URL + "/page/tgn/7029392",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			location, err := GetLocationFromTGN(tc.URI)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			t.Logf("%s -> %+v", tc.URI, location)

			if *location != tc.Expected {
				t.Errorf("expected %+v, got %+v", tc.Expected, location)
			}
		})
	}
}

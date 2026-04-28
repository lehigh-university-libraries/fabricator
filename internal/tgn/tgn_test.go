package tgn

import (
	"testing"
)

func TestGetLocationFromTGN(t *testing.T) {
	tests := map[string]struct {
		URI      string
		Expected *Location
	}{
		"Test Bethlehem": {
			URI: "http://vocab.getty.edu/page/tgn/7013416",
			Expected: &Location{
				Country:     "United States",
				State:       "Pennsylvania",
				County:      "Northampton",
				City:        "Bethlehem",
				Coordinates: "40.6167,-75.35",
			},
		},
		"Test Coplay": {
			URI: "http://vocab.getty.edu/page/tgn/2087483",
			Expected: &Location{
				Country:     "United States",
				State:       "Pennsylvania",
				County:      "Lehigh",
				City:        "Coplay",
				Coordinates: "40.6667,-75.4833",
			},
		},
		"Test Luxembourg": {
			URI: "http://vocab.getty.edu/page/tgn/7003514",
			// Top-level continental record: no county/state, but should still
			// have a country, city/place label, and coordinates.
			Expected: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			location, err := GetLocationFromTGN(tc.URI)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			t.Logf("%s -> %+v", tc.URI, location)

			if tc.Expected != nil && *location != *tc.Expected {
				t.Errorf("expected %+v, got %+v", tc.Expected, location)
			}

			if location.Coordinates == "" {
				t.Errorf("expected non-empty Coordinates for %s", tc.URI)
			}
		})
	}
}

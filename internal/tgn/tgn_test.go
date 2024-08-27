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
				Country: "United States",
				State:   "Pennsylvania",
				County:  "Northampton",
				City:    "Bethlehem",
			},
		},
		"Test Coplay": {
			URI: "http://vocab.getty.edu/page/tgn/2087483",
			Expected: &Location{
				Country: "United States",
				State:   "Pennsylvania",
				County:  "Lehigh",
				City:    "Coplay",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			location, err := GetLocationFromTGN(tc.URI)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if *location != *tc.Expected {
				t.Errorf("expected %+v, got %+v", tc.Expected, location)
			}
		})
	}
}

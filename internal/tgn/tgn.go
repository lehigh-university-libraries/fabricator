package tgn

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// Location represents the hierarchical location details stored in Islandora
type Location struct {
	Country     string `json:"country"`
	State       string `json:"state"`
	County      string `json:"county"`
	City        string `json:"city"`
	Coordinates string `json:"-"`
}

// Place represents the TGN data
type Place struct {
	ID           string `json:"id"`
	Label        string `json:"_label"`
	ClassifiedAs []struct {
		ID string `json:"id"`
	} `json:"classified_as"`
	PartOf []struct {
		ID    string `json:"id"`
		Label string `json:"_label"`
	} `json:"part_of"`
	IdentifiedBy []struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"identified_by"`
}

// extractCoordinates pulls "lat,lng" out of the GeoJSON-style "[lng,lat]"
// value carried on a crm:E47_Spatial_Coordinates entry. Returns "" when no
// coordinate entry is present or it cannot be parsed.
func extractCoordinates(p Place) string {
	for _, ib := range p.IdentifiedBy {
		if ib.Type != "crm:E47_Spatial_Coordinates" {
			continue
		}
		raw := strings.TrimSpace(ib.Value)
		raw = strings.TrimPrefix(raw, "[")
		raw = strings.TrimSuffix(raw, "]")
		parts := strings.Split(raw, ",")
		if len(parts) != 2 {
			return ""
		}
		lng, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			return ""
		}
		lat, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return ""
		}
		return fmt.Sprintf("%g,%g", lat, lng)
	}
	return ""
}

// canonicalJSONURI rewrites a TGN URI of any common form (page/tgn/{id},
// tgn/{id}, with or without trailing slash or .json) into the canonical
// vocab.getty.edu/tgn/{id}.json endpoint that consistently returns JSON.
func canonicalJSONURI(uri string) string {
	u := strings.TrimSpace(uri)
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".json")
	u = strings.Replace(u, "/page/tgn/", "/tgn/", 1)
	return u + ".json"
}

// GetLocationFromTGN fetches the location information from a TGN URI.
func GetLocationFromTGN(uri string) (*Location, error) {
	req, err := http.NewRequest("GET", canonicalJSONURI(uri), nil)
	if err != nil {
		slog.Error("Unable to create hierarchy request", "url", uri, "err", err)
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching data: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	// Parse the JSON data
	var place Place
	if err := json.Unmarshal(body, &place); err != nil {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("error parsing JSON (status %d, content-type %q): %v; body: %s",
			resp.StatusCode, resp.Header.Get("Content-Type"), err, preview)
	}

	location := &Location{
		Coordinates: extractCoordinates(place),
	}

	// Recursively process the hierarchy
	err = resolveHierarchy(place, location)
	if err != nil {
		return nil, err
	}

	return location, nil
}

// resolveHierarchy recursively resolves the hierarchy from the place data.
func resolveHierarchy(place Place, location *Location) error {
	if len(place.PartOf) > 0 {
		parentPlace, err := fetchPlaceData(place.PartOf[0].ID + ".json")
		if err != nil {
			return err
		}
		if err := resolveHierarchy(parentPlace, location); err != nil {
			return err
		}
	}

	// AAT place types identify administrative levels regardless of hierarchy depth.
	// Unmapped types (including continents and World) do not fill these fields.
	for _, classification := range place.ClassifiedAs {
		switch strings.Replace(classification.ID, "https://", "http://", 1) {
		case "http://vocab.getty.edu/aat/300128207": // nations
			location.Country = place.Label
		case "http://vocab.getty.edu/aat/300387064", // first level subdivisions
			"http://vocab.getty.edu/aat/300000776", // states
			"http://vocab.getty.edu/aat/300000774": // provinces
			location.State = place.Label
		case "http://vocab.getty.edu/aat/300387145", // second level subdivisions
			"http://vocab.getty.edu/aat/300000771": // counties
			location.County = place.Label
		case "http://vocab.getty.edu/aat/300008347", // inhabited places
			"http://vocab.getty.edu/aat/300008389": // cities
			location.City = place.Label
		}
	}

	return nil
}

// fetchPlaceData fetches the JSON data for a given TGN URI.
func fetchPlaceData(uri string) (Place, error) {
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		slog.Error("Unable to create hierarchy request", "url", uri, "err", err)
		return Place{}, err
	}

	req.Header.Set("Accept", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Place{}, fmt.Errorf("error fetching data: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Place{}, fmt.Errorf("error reading response body: %v", err)
	}

	var place Place
	if err := json.Unmarshal(body, &place); err != nil {
		return Place{}, fmt.Errorf("error parsing JSON: %v", err)
	}

	return place, nil
}

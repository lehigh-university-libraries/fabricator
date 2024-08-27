package tgn

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// Location represents the hierarchical location details stored in Islandora
type Location struct {
	Country string `json:"country"`
	State   string `json:"state"`
	County  string `json:"county"`
	City    string `json:"city"`
}

// Place represents the TGN data
type Place struct {
	ID     string `json:"id"`
	Label  string `json:"_label"`
	PartOf []struct {
		ID    string `json:"id"`
		Label string `json:"_label"`
	} `json:"part_of"`
}

// GetLocationFromTGN fetches the location information from a TGN URI.
func GetLocationFromTGN(uri string) (*Location, error) {
	req, err := http.NewRequest("GET", uri, nil)
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
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	location := &Location{}

	// Recursively process the hierarchy
	err = resolveHierarchy(place, location, 0)
	if err != nil {
		return nil, err
	}

	return location, nil
}

// resolveHierarchy recursively resolves the hierarchy from the place data.
func resolveHierarchy(place Place, location *Location, depth int) error {
	// If this place has no parent, it must be the city
	if len(place.PartOf) == 0 {
		location.Country = place.Label
		return nil
	}

	// Recursively resolve the parent hierarchy
	parentPlace, err := fetchPlaceData(place.PartOf[0].ID + ".json")
	if err != nil {
		return err
	}
	err = resolveHierarchy(parentPlace, location, depth+1)
	if err != nil {
		return err
	}

	// Assign the correct label based on depth
	switch depth {
	case 0:
		location.City = place.Label
	case 1:
		location.County = place.Label
	case 2:
		location.State = place.Label
	case 3:
		location.Country = place.Label
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

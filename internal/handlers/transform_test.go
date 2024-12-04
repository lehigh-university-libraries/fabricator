package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadCSVWithJSONTags(t *testing.T) {
	tests := []struct {
		name            string
		csvContent      string
		expectedHeaders []string
		expectedRows    []map[string][]string
		expectedAgents  [][]string
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
			expectedAgents: nil,
			expectError:    false,
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
			expectedAgents: nil,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate HTTP request with CSV body
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.csvContent))
			req.Header.Set("Content-Type", "text/csv")

			// Call function under test
			headers, rows, _, err := readCSVWithJSONTags(req)
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

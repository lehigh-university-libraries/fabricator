package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCheckMyWork(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       interface{}
		statusCode int
		response   string
	}{
		{
			name:       "Method not allowed",
			method:     http.MethodGet,
			body:       nil,
			statusCode: http.StatusMethodNotAllowed,
			response:   "Method not allowed\n",
		},
		{
			name:       "Not authorized",
			method:     http.MethodPost,
			body:       nil,
			statusCode: http.StatusUnauthorized,
			response:   "Authorization header missing\n",
		},
		{
			name:       "Empty request body",
			method:     http.MethodPost,
			body:       nil,
			statusCode: http.StatusBadRequest,
			response:   "Request body is empty\n",
		},
		{
			name:       "Invalid JSON body",
			method:     http.MethodPost,
			body:       "invalid_json",
			statusCode: http.StatusBadRequest,
			response:   "Error parsing CSV\n",
		},
		{
			name:       "No rows in CSV to process",
			method:     http.MethodPost,
			body:       [][]string{{"Title", "Object Model"}},
			statusCode: http.StatusOK,
			response:   `{"A":"No rows in CSV to process"}`,
		},
		{
			name:   "Valid CSV data with no errors",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title"},
				{"Test Title", "Model", "Full Test Title"},
			},
			statusCode: http.StatusOK,
			response:   "{}", // Empty errors map
		},
		{
			name:   "Missing title",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title"},
				{"", "Model", "foo"},
			},
			statusCode: http.StatusOK,
			response:   `{"A2":"Missing value"}`,
		},
		{
			name:   "Missing model",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title"},
				{"foo", "", "bar"},
			},
			statusCode: http.StatusOK,
			response:   `{"B2":"Missing value"}`,
		},
		{
			name:   "Missing full title",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title"},
				{"Test Title", "Model", ""},
			},
			statusCode: http.StatusOK,
			response:   `{"C2":"Missing value"}`,
		},
	}

	sharedSecret := "foo"
	os.Setenv("SHARED_SECRET", sharedSecret)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody *bytes.Buffer
			if tt.body != nil {
				bodyBytes, err := json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("Failed to marshal body: %v", err)
				}
				reqBody = bytes.NewBuffer(bodyBytes)
			} else {
				reqBody = bytes.NewBuffer(nil)
			}

			req := httptest.NewRequest(tt.method, "/check-my-work", reqBody)
			if tt.name == "Not authorized" {
				req.Header.Set("X-Secret", "nope")
			} else {
				req.Header.Set("X-Secret", sharedSecret)
			}
			rec := httptest.NewRecorder()

			// Call the handler
			CheckMyWork(rec, req)

			// Assert response status code
			if rec.Code != tt.statusCode {
				t.Errorf("Expected status code %d, got %d", tt.statusCode, rec.Code)
			}

			// Assert response body
			if rec.Body.String() != tt.response {
				t.Errorf("Expected response body: %q, got %q", tt.response, rec.Body.String())
			}
		})
	}
}

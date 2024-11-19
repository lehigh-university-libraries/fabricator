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
	files := []struct {
		name         string
		permissions  os.FileMode
		expectAccess bool
	}{
		{"/tmp/test_readable.txt", 0644, true}, // Readable globally
		{"/tmp/test_writable.txt", 0666, true}, // Writable globally
		{"/tmp/test_private.txt", 0600, false}, // Not accessible globally
	}

	// Create test files
	for _, file := range files {
		if err := os.WriteFile(file.name, []byte("test content"), file.permissions); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file.name, err)
		}
	}
	defer func() {
		for _, file := range files {
			_ = os.Remove(file.name)
		}
	}()

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
				{"foo", "bar", "Full Test Title"},
			},
			statusCode: http.StatusOK,
			response:   "{}", // Empty errors map
		},
		{
			name:   "Missing title",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title"},
				{"", "bar", "foo"},
			},
			statusCode: http.StatusOK,
			response:   `{"A2":"Missing value"}`,
		},
		{
			name:   "Missing resource type",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Resource Type"},
				{"foo", "bar", "foo", ""},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"Missing value"}`,
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
				{"foo", "bar", ""},
			},
			statusCode: http.StatusOK,
			response:   `{"C2":"Missing value"}`,
		},
		{
			name:   "Non-existent file",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "File Path"},
				{"foo", "bar", "foo", "/tmp/file/does/not/exist"},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"File does not exist in islandora_staging"}`,
		},
		{
			name:   "Missing file",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "File Path"},
				{"foo", "Image", "foo", ""},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"Missing source file"}`,
		},
		{
			name:   "OK file",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "File Path"},
				{"foo", "Image", "foo", "/tmp/test_readable.txt"},
			},
			statusCode: http.StatusOK,
			response:   `{}`,
		},
		{
			name:   "OK file (rw)",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "File Path"},
				{"foo", "Image", "foo", "/tmp/test_writable.txt"},
			},
			statusCode: http.StatusOK,
			response:   `{}`,
		},

		{
			name:   "Unreadable file",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "File Path"},
				{"foo", "Image", "foo", "/tmp/test_private.txt"},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"File does not exist in islandora_staging"}`,
		},
		{
			name:   "Missing file OK",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "File Path"},
				{"foo", "Paged Content", "foo", ""},
			},
			statusCode: http.StatusOK,
			response:   `{}`,
		},
		{
			name:   "Missing supplemental file",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Supplemental File"},
				{"foo", "bar", "foo", "/tmp/file/does/not/exist"},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"File does not exist in islandora_staging"}`,
		},

		// Parent Collection and PPI
		{
			name:   "Parent Collection not integer",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Parent Collection"},
				{"foo", "bar", "foo", "NotAnInteger"},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"Must be an integer"}`,
		},
		{
			name:   "Parent Collection not found",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Parent Collection"},
				{"foo", "bar", "foo", "0"},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"Could not identify parent collection 0"}`,
		},
		{
			name:   "Parent Collection exists",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Parent Collection"},
				{"foo", "bar", "foo", "2"},
			},
			statusCode: http.StatusOK,
			response:   `{}`,
		},
		{
			name:   "Invalid URL",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Catalog or ArchivesSpace URL"},
				{"foo", "bar", "foo", "invalid-url"},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"Invalid URL"}`,
		},
		{
			name:   "Duplicate Upload ID",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Upload ID"},
				{"Test Title 1", "bar", "foo", "123"},
				{"Test Title 2", "bar", "foo", "123"},
			},
			statusCode: http.StatusOK,
			response:   `{"D3":"Duplicate upload ID"}`,
		},
		{
			name:   "Invalid EDTF value",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Creation Date"},
				{"foo", "bar", "foo", "1/2/2022"},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"Invalid EDTF value"}`,
		},
		{
			name:   "Invalid DOI",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "DOI"},
				{"foo", "bar", "foo", "1.2.3.4"},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"Invalid DOI"}`,
		},
		{
			name:   "Unknown Parent ID",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Page/Item Parent ID", "Upload ID"},
				{"foo", "bar", "foo", "123", "456"},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"Unknown parent ID"}`,
		},
		{
			name:   "Upload ID equals Parent ID",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Upload ID", "Page/Item Parent ID"},
				{"foo", "bar", "foo", "123", "123"},
			},
			statusCode: http.StatusOK,
			response:   `{"E2":"Upload ID and parent ID can not be equal"}`,
		},
		// Contributor
		{
			name:   "Contributor not in proper format",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Contributor"},
				{"foo", "bar", "foo", `{"name":"bad-format"}`},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"Contributor name not in proper format"}`,
		},
		{
			name:   "Paged Content need collection",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Parent Collection"},
				{"foo", "Paged Content", "foo", ""},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"Paged content must have a parent collection"}`,
		},
		{
			name:   "Page needs Parent ID",
			method: http.MethodPost,
			body: [][]string{
				{"Title", "Object Model", "Full Title", "Page/Item Parent ID"},
				{"foo", "Page", "foo", ""},
			},
			statusCode: http.StatusOK,
			response:   `{"D2":"Pages must have a parent id"}`,
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

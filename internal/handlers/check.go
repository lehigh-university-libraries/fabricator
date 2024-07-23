package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/jwk"
	edtf "github.com/sfomuseum/go-edtf/parser"
)

const googleCertsURL = "https://www.googleapis.com/oauth2/v3/certs"

func CheckMyWork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		slog.Error("Authorization header missing")
		http.Error(w, "Authorization header missing", http.StatusUnauthorized)
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		slog.Error("Token not found")
		http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
		return
	}

	tokenString := parts[1]
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("kid not found in token header")
		}

		ctx := context.Background()
		jwksSet, err := jwk.Fetch(ctx, googleCertsURL)
		if err != nil {
			return nil, fmt.Errorf("unable to fetch JWK set from %s: %v", googleCertsURL, err)
		}
		key, ok := jwksSet.LookupKeyID(kid)
		if !ok {
			return nil, fmt.Errorf("unable to find key '%s'", kid)
		}

		var pubkey interface{}
		if err := key.Raw(&pubkey); err != nil {
			return nil, fmt.Errorf("failed to get raw key: %v", err)
		}

		return pubkey, nil
	})

	if err != nil || !token.Valid {
		slog.Error("Unable to validate token", "err", err)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	claims := token.Claims.(jwt.MapClaims)
	emailVerified, ok := claims["email_verified"].(bool)
	if !emailVerified || !ok {
		slog.Error("Unverified email", "err", err)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	email, ok := claims["email"].(string)
	if !ok || len(email) < 11 || email[len(email)-11:] != "@lehigh.edu" {
		http.Error(w, "Error extracting email from token", http.StatusInternalServerError)
		return
	}

	if r.ContentLength == 0 {
		http.Error(w, "Request body is empty", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	var csvData [][]string
	err = json.NewDecoder(r.Body).Decode(&csvData)
	if err != nil {
		slog.Error("Error parsing CSV", "err", err)
		http.Error(w, "Error parsing CSV", http.StatusBadRequest)
		return
	}

	if len(csvData) < 2 {
		http.Error(w, "No rows in CSV to process", http.StatusBadRequest)
	}

	header := csvData[0]
	doiPattern := regexp.MustCompile(`^10\.\d{4,9}\/[-._;()/:A-Za-z0-9]+$`)
	gettyTgnPattern := regexp.MustCompile(`^http://vocab\.getty\.edu/page/tgn/\d+$`)
	datePattern := regexp.MustCompile(`^\d{4}(-\d{2}(-\d{2})?)?$`)

	errors := map[string]string{}
	requiredFields := []string{
		"Title",
		"Object Model",
	}
	uploadIds := map[string]bool{}
	for rowIndex, row := range csvData[1:] {
		for colIndex, col := range row {
			column := header[colIndex]
			c := numberToExcelColumn(colIndex)
			i := c + strconv.Itoa(rowIndex+2)
			if col == "" {
				if strInSlice(column, requiredFields) {
					errors[i] = "Missing value"
				}

				continue
			}

			for _, cell := range strings.Split(col, " ; ") {

				switch column {
				// make sure these columns are integers
				case "Parent Collection", "PPI":
					_, err := strconv.Atoi(cell)
					if err != nil {
						errors[i] = "Must be an integer"
					}
				// make sure these columns are valid URLs
				case "Catalog or ArchivesSpace URL":
					parsedURL, err := url.ParseRequestURI(cell)
					if err != nil || parsedURL.Scheme == "" && parsedURL.Host == "" {
						errors[i] = "Invalid URL"
					}
				// make sure each upload ID is unique
				case "Upload ID":
					if _, exists := uploadIds[cell]; exists {
						errors[i] = "Duplicate upload ID"
					}
					uploadIds[cell] = true
				// check for valid EDTF values
				case "Creation Date", "Date Captured", "Embargo Until Date":
					if !datePattern.MatchString(cell) && !edtf.IsValid(cell) {
						errors[i] = "Invalid EDTF value"
					}
				// check for valid DOI value
				case "DOI":
					if !doiPattern.MatchString(cell) {
						errors[i] = "Invalid DOI"
					}
				// make sure the parent ID matches an upload ID in the spreadsheet
				case "Page/Item Parent ID":
					if _, ok := uploadIds[cell]; !ok {
						errors[i] = "Unknown parent ID"
					}
				// make sure the file exists in the filesystem
				case "File Path":
					filename := strings.ReplaceAll(cell, `\`, `/`)
					filename = strings.TrimLeft(filename, "/")
					if len(filename) > 3 && filename[0:3] != "mnt" {
						filename = fmt.Sprintf("/mnt/islandora_staging/%s", filename)
					}

					filename = strings.ReplaceAll(filename, "/mnt/islandora_staging", "/data")
					if !fileExists(filename) {
						errors[i] = "File does not exist in islandora_staging"
					}
				case "Subject Geographic (LCNAF)":
					if !gettyTgnPattern.MatchString(cell) {
						errors[i] = "Invalid Getty TGN URI"
					}
					hierarchyURL := strings.Replace(cell, "page", "hierarchy", 1)

					req, err := http.NewRequest("GET", hierarchyURL, nil)
					if err != nil {
						break
					}
					req.Header.Set("Accept", "application/json")

					client := &http.Client{}
					resp, err := client.Do(req)
					if err != nil {
						slog.Error("Unable to request hierarchy URL", "url", hierarchyURL, "err", err)
						errors[i] = "Unable to request hierarchical information"
						break
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						slog.Error("Unable to get hierarchy URL", "url", hierarchyURL, "err", err)
						errors[i] = "Unable to get hierarchical information"
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	jsonResponse, err := json.Marshal(errors)
	if err != nil {
		slog.Error("Error creating JSON response", "err", err)
		http.Error(w, "Error creating JSON response", http.StatusInternalServerError)
		return
	}

	_, err = w.Write(jsonResponse)
	if err != nil {
		slog.Error("Error writing JSON response", "err", err)
		http.Error(w, "Error writing JSON response", http.StatusInternalServerError)
	}
}

func strInSlice(s string, sl []string) bool {
	for _, a := range sl {
		if a == s {
			return true
		}
	}
	return false
}

func numberToExcelColumn(n int) string {
	result := ""
	for {
		char := 'A' + rune(n%26)
		result = string(char) + result
		n = n/26 - 1
		if n < 0 {
			break
		}
	}
	return result
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

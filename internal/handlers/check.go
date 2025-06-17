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
	"sync"
	"time"

	"github.com/lehigh-university-libraries/fabricator/internal/contributor"
	"github.com/lehigh-university-libraries/fabricator/internal/tgn"
	"github.com/lestrrat-go/jwx/v3/jwk"
	jwt "github.com/lestrrat-go/jwx/v3/jwt"
	edtf "github.com/sfomuseum/go-edtf/parser"
)

const googleCertsURL = "https://www.googleapis.com/oauth2/v3/certs"

func CheckMyWork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !authRequest(w, r) {
		return
	}

	if r.ContentLength == 0 {
		http.Error(w, "Request body is empty", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	var csvData [][]string
	err := json.NewDecoder(r.Body).Decode(&csvData)
	if err != nil {
		slog.Error("Error parsing CSV", "err", err)
		http.Error(w, "Error parsing CSV", http.StatusBadRequest)
		return
	}

	errors := map[string]string{}
	if len(csvData) < 2 {
		errorColumn := numberToExcelColumn(0)
		errors[errorColumn] = "No rows in CSV to process"
		csvData = append(csvData, []string{})
	}

	relators := validRelators()

	header := csvData[0]
	doiPattern := regexp.MustCompile(`^10\.\d{4,9}\/[-._;()/:A-Za-z0-9]+$`)
	datePattern := regexp.MustCompile(`^\d{4}(-\d{2}(-\d{2})?)?$`)
	hierarchyChecked := map[string]bool{}
	requiredFields := []string{
		"Title",
		"Object Model",
		"Full Title",
	}
	urlCheckCache := &sync.Map{}
	uploadIds := map[string]bool{}
	for rowIndex, row := range csvData[1:] {
		for colIndex, col := range row {
			column := header[colIndex]
			c := numberToExcelColumn(colIndex)
			i := c + strconv.Itoa(rowIndex+2)
			if col == "" {
				// require fields on create
				if strInSlice(column, requiredFields) && ColumnValue("Node ID", header, row) == "" {
					errors[i] = "Missing value"
				}
				if column == "Parent Collection" {
					model := ColumnValue("Object Model", header, row)
					if model == "Paged Content" && ColumnValue("Page/Item Parent ID", header, row) == "" {
						errors[i] = "Paged content must have a parent collection or parent ID"
					}
				}

				if column == "Page/Item Parent ID" {
					model := ColumnValue("Object Model", header, row)
					if model == "Page" && ColumnValue("Parent Collection", header, row) == "" {
						errors[i] = "Pages must have a parent id or parent collection"
					}
				}

				if column == "Resource Type" {
					model := ColumnValue("Object Model", header, row)
					if model != "Page" {
						errors[i] = "Must have a resource type"
					}
				}

				continue
			}

			for _, cell := range strings.Split(col, " ; ") {
				cell = strings.TrimSpace(cell)
				if cell == "" {
					continue
				}

				switch column {
				// make sure these columns are integers
				case "Parent Collection", "PPI", "Node ID":
					id, err := strconv.Atoi(cell)
					if err != nil {
						errors[i] = "Must be an integer"
						break
					}
					if column == "Parent Collection" {
						url := fmt.Sprintf("%s/node/%d?_format=json", os.Getenv("ISLE_SITE_URL"), id)
						if !checkURL(url, urlCheckCache) {
							errors[i] = fmt.Sprintf("Could not identify parent collection %d", id)
						}
					}
					if column == "Node ID" {
						url := fmt.Sprintf("%s/node/%d?_format=json", os.Getenv("ISLE_SITE_URL"), id)
						if !checkURL(url, urlCheckCache) {
							errors[i] = fmt.Sprintf("Could not find node ID %d", id)
						}
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
						slog.Error("Invalid EDTF value", "cell", cell)
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
						break
					}
					id := ColumnValue("Upload ID", header, row)
					if cell == id {
						errors[i] = "Upload ID and parent ID can not be equal"
					}
				case "Contributor":
					var c contributor.Contributor
					err := json.Unmarshal([]byte(cell), &c)
					if err != nil {
						errors[i] = "Contributor not in proper format"
					}
					name := strings.Split(c.Name, ":")
					if len(name) < 4 {
						errors[i] = "Contributor name not in proper format"
						break
					}
					if c.Status != "" || c.Email != "" || c.Institution != "" || c.Orcid != "" {
						if name[2] != "person" {
							errors[i] = "Additional fields can only be applied to people"
						}
					}
					if !strInSlice(name[2], []string{"person", "corporate_body"}) {
						errors[i] = fmt.Sprintf("Bad vocabulary ID for contributor: %s", name[2])
					}
					relator := fmt.Sprintf("%s:%s", name[0], name[1])
					if !strInSlice(relator, relators) {
						errors[i] = fmt.Sprintf("Invalid relator: %s", relator)
					}

					// make sure the file exists in the filesystem
				case "File Path", "Supplemental File":
					filename := strings.ReplaceAll(cell, `\`, `/`)

					if len(filename) > 7 && filename[0:6] == "/home/" {
						break
					} else if len(filename) > 3 && filename[0:3] != "mnt" {
						filename = strings.TrimLeft(filename, "/")
						filename = fmt.Sprintf("/mnt/islandora_staging/%s", filename)
					}

					filename = strings.ReplaceAll(filename, "/mnt/islandora_staging", os.Getenv("FABRICATOR_DATA_MOUNT"))
					if !fileExists(filename) {
						errors[i] = "File does not exist in islandora_staging"
					}
				case "Add Coverpage (Y/N)", "Make Public (Y/N)":
					if cell != "Yes" && cell != "No" {
						errors[i] = "Invalid value. Must be Yes or No"
					}
				case "Hierarchical Geographic (Getty TGN)":
					if hierarchyChecked[cell] {
						break
					}
					hierarchyChecked[cell] = true
					_, err := tgn.GetLocationFromTGN(cell)
					if err != nil {
						errors[i] = "Unable to get TGN"
					}
				case "Title":
					if len(cell) > 255 {
						errors[i] = "Title is longer than 255 characters"
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
	if !info.Mode().IsRegular() {
		return false
	}

	mode := info.Mode().Perm()
	// Check if the file is globally readable
	return mode&0004 != 0
}

func authRequest(w http.ResponseWriter, r *http.Request) bool {
	secret := r.Header.Get("X-Secret")
	if secret == os.Getenv("SHARED_SECRET") {
		return true
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		slog.Error("Authorization header missing")
		http.Error(w, "Authorization header missing", http.StatusUnauthorized)
		return false
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		slog.Error("Token not found")
		http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
		return false
	}

	tokenString := parts[1]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	keySet, err := jwk.Fetch(ctx, googleCertsURL)
	if err != nil {
		slog.Error("unable to fetch JWK set", "url", googleCertsURL, "err", err)
	}

	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithKeySet(keySet),
		jwt.WithValidate(true),
		jwt.WithContext(ctx),
	)
	if err != nil {
		slog.Error("unable to parse token", "err", err)
		return false
	}

	var emailVerified bool
	err = token.Get("email_verified", &emailVerified)
	if err != nil || !emailVerified {
		slog.Error("Unverified email", "err", err)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return false
	}

	var email string
	err = token.Get("email", &email)
	if err != nil {
		slog.Error("No email claim", "err", err)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return false
	}
	if len(email) < 11 || email[len(email)-11:] != "@lehigh.edu" {
		slog.Error("Unknown email", "email", email)
		http.Error(w, "Error extracting email from token", http.StatusInternalServerError)
		return false
	}

	return true
}

func IndexOf(value string, slice []string) int {
	for i, v := range slice {
		if v == value {
			return i
		}
	}
	return -1
}

func ColumnValue(value string, header, row []string) string {
	i := IndexOf(value, header)
	if i == -1 {
		return ""
	}

	return row[i]
}

func checkURL(url string, cache *sync.Map) bool {
	if result, ok := cache.Load(url); ok {
		return result.(bool)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Head(url)
	if err != nil {
		cache.Store(url, false)
		return false
	}
	defer resp.Body.Close()

	result := resp.StatusCode == http.StatusOK
	cache.Store(url, result)
	return result
}

func validRelators() []string {
	// todo: fetch this from field_linked_agent config
	return []string{
		"relators:att",
		"relators:abr",
		"relators:act",
		"relators:adp",
		"relators:rcp",
		"relators:anl",
		"relators:anm",
		"relators:ann",
		"relators:apl",
		"relators:ape",
		"relators:app",
		"relators:arc",
		"relators:arr",
		"relators:acp",
		"relators:adi",
		"relators:art",
		"relators:ard",
		"relators:asn",
		"relators:asg",
		"relators:auc",
		"relators:aut",
		"relators:aqt",
		"relators:aft",
		"relators:aud",
		"relators:aui",
		"relators:ato",
		"relators:ant",
		"relators:bnd",
		"relators:bdd",
		"relators:blw",
		"relators:bkd",
		"relators:bkp",
		"relators:bjd",
		"relators:bpd",
		"relators:bsl",
		"relators:brl",
		"relators:brd",
		"relators:cll",
		"relators:ctg",
		"relators:cas",
		"relators:cns",
		"relators:chr",
		"relators:clb",
		"relators:cng",
		"relators:cli",
		"relators:cor",
		"relators:col",
		"relators:clt",
		"relators:clr",
		"relators:cmm",
		"relators:cwt",
		"relators:com",
		"relators:cpl",
		"relators:cpt",
		"relators:cpe",
		"relators:cmp",
		"relators:cmt",
		"relators:ccp",
		"relators:cnd",
		"relators:con",
		"relators:csl",
		"relators:csp",
		"relators:cos",
		"relators:cot",
		"relators:coe",
		"relators:cts",
		"relators:ctt",
		"relators:cte",
		"relators:ctr",
		"relators:ctb",
		"relators:cpc",
		"relators:cph",
		"relators:crr",
		"relators:crp",
		"relators:cst",
		"relators:cou",
		"relators:crt",
		"relators:cov",
		"relators:cre",
		"relators:cur",
		"relators:dnc",
		"relators:dtc",
		"relators:dtm",
		"relators:dte",
		"relators:dto",
		"relators:dfd",
		"relators:dft",
		"relators:dfe",
		"relators:dgg",
		"relators:dgs",
		"relators:dln",
		"relators:dpc",
		"relators:dpt",
		"relators:dsr",
		"relators:drt",
		"relators:dis",
		"relators:dbp",
		"relators:dst",
		"relators:dnr",
		"relators:drm",
		"relators:dub",
		"relators:edt",
		"relators:edc",
		"relators:edm",
		"relators:edd",
		"relators:elg",
		"relators:elt",
		"relators:enj",
		"relators:eng",
		"relators:egr",
		"relators:etr",
		"relators:evp",
		"relators:exp",
		"relators:fac",
		"relators:fld",
		"relators:fmd",
		"relators:fds",
		"relators:flm",
		"relators:fmp",
		"relators:fmk",
		"relators:fpy",
		"relators:frg",
		"relators:fmo",
		"relators:fnd",
		"relators:gis",
		"relators:grt",
		"relators:hnr",
		"relators:hst",
		"relators:his",
		"relators:ilu",
		"relators:ill",
		"relators:ins",
		"relators:itr",
		"relators:ive",
		"relators:ivr",
		"relators:inv",
		"relators:isb",
		"relators:jud",
		"relators:jug",
		"relators:lbr",
		"relators:ldr",
		"relators:lsa",
		"relators:led",
		"relators:len",
		"relators:lil",
		"relators:lit",
		"relators:lie",
		"relators:lel",
		"relators:let",
		"relators:lee",
		"relators:lbt",
		"relators:lse",
		"relators:lso",
		"relators:lgd",
		"relators:ltg",
		"relators:lyr",
		"relators:mfp",
		"relators:mfr",
		"relators:mrb",
		"relators:mrk",
		"relators:med",
		"relators:mdc",
		"relators:mte",
		"relators:mtk",
		"relators:mod",
		"relators:mon",
		"relators:mcp",
		"relators:msd",
		"relators:mus",
		"relators:nrt",
		"relators:osp",
		"relators:opn",
		"relators:orm",
		"relators:org",
		"relators:oth",
		"relators:own",
		"relators:pan",
		"relators:ppm",
		"relators:pta",
		"relators:pth",
		"relators:pat",
		"relators:prf",
		"relators:pma",
		"relators:pht",
		"relators:ptf",
		"relators:ptt",
		"relators:pte",
		"relators:plt",
		"relators:pra",
		"relators:pre",
		"relators:prt",
		"relators:pop",
		"relators:prm",
		"relators:prc",
		"relators:pro",
		"relators:prn",
		"relators:prs",
		"relators:pmn",
		"relators:prd",
		"relators:prp",
		"relators:prg",
		"relators:pdr",
		"relators:pfr",
		"relators:prv",
		"relators:pup",
		"relators:pbd",
		"relators:ppt",
		"relators:rdd",
		"relators:rpc",
		"relators:rce",
		"relators:rcd",
		"relators:red",
		"relators:ren",
		"relators:rpt",
		"relators:rps",
		"relators:rth",
		"relators:rtm",
		"relators:res",
		"relators:rsp",
		"relators:rst",
		"relators:rse",
		"relators:rpy",
		"relators:rsg",
		"relators:rsr",
		"relators:rev",
		"relators:rbr",
		"relators:sce",
		"relators:sad",
		"relators:aus",
		"relators:scr",
		"relators:scl",
		"relators:spy",
		"relators:sec",
		"relators:sll",
		"relators:std",
		"relators:stg",
		"relators:sgn",
		"relators:sng",
		"relators:sds",
		"relators:spk",
		"relators:spn",
		"relators:sgd",
		"relators:stm",
		"relators:stn",
		"relators:str",
		"relators:stl",
		"relators:sht",
		"relators:srv",
		"relators:tch",
		"relators:tcd",
		"relators:tld",
		"relators:tlp",
		"relators:ths",
		"relators:trc",
		"relators:trl",
		"relators:tyd",
		"relators:tyg",
		"relators:uvp",
		"relators:vdg",
		"relators:voc",
		"relators:vac",
		"relators:wit",
		"relators:wde",
		"relators:wdc",
		"relators:wam",
		"relators:wac",
		"relators:wal",
		"relators:wat",
		"relators:win",
		"relators:wpr",
		"relators:wst",
		"label:department",
	}
}

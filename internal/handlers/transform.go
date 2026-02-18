package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/lehigh-university-libraries/fabricator/internal/contributor"
	"github.com/lehigh-university-libraries/fabricator/internal/tgn"
	"github.com/lehigh-university-libraries/go-islandora/workbench"
)

func TransformCsv(w http.ResponseWriter, r *http.Request) {
	headers, rows, err := readCSVWithJSONTags(r)
	if err != nil {
		slog.Error("Failed to read CSV", "err", err)
		http.Error(w, "Error parsing CSV", http.StatusBadRequest)
		return
	}

	firstRow := make([]string, 0, len(headers))
	for header := range headers {
		firstRow = append(firstRow, header)
	}
	target := "/tmp/target.csv"
	if strInSlice("node_id", firstRow) {
		target = "/tmp/target.update.csv"
	}
	file, err := os.Create(target)
	if err != nil {
		slog.Error("Failed to create file", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	writer := csv.NewWriter(file)

	// finally, write the header to the CSV
	if err := writer.Write(firstRow); err != nil {
		slog.Error("Failed to write record to CSV", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// write the rows to the CSV
	for _, row := range rows {
		record := []string{}
		for _, header := range firstRow {
			record = append(record, strings.Join(row[header], "|"))
		}
		if err := writer.Write(record); err != nil {
			slog.Error("Failed to write record to CSV", "err", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
	}
	writer.Flush()
	file.Close()

	files := []string{
		target,
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=files.zip")

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Iterate over each file path in the slice and add it to the zip archive
	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			http.Error(w, "Error opening file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		fileName := filepath.Base(filePath)
		zipFile, err := zipWriter.Create(fileName)
		if err != nil {
			http.Error(w, "Error creating zip entry: "+err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = io.Copy(zipFile, file)
		if err != nil {
			http.Error(w, "Error writing to zip: "+err.Error(), http.StatusInternalServerError)
			return
		}
		file.Close()
		os.Remove(filePath)
		w.(http.Flusher).Flush()
	}

}

func getJSONFieldName(tag string) string {
	if commaIndex := strings.Index(tag, ","); commaIndex != -1 {
		return tag[:commaIndex]
	}
	return tag
}

func readCSVWithJSONTags(r *http.Request) (map[string]bool, []map[string][]string, error) {
	defer r.Body.Close()
	re := regexp.MustCompile(`^\d{1,4}$`)
	reader := csv.NewReader(r.Body)
	headers, err := reader.Read()
	if err != nil {
		return nil, nil, err
	}

	var rows []map[string][]string
	newHeaders := map[string]bool{}

	resolver := newDrupalTermResolver()
	newCsv := &workbench.SheetsCsv{}
	tgnCache := make(map[string]string)
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}

		row := map[string][]string{}
		v := reflect.ValueOf(newCsv).Elem()
		t := v.Type()

		for i, header := range headers {
			for j := 0; j < t.NumField(); j++ {
				field := t.Field(j)
				jsonTag := getJSONFieldName(field.Tag.Get("json"))
				if jsonTag != header {
					continue
				}
				value := v.FieldByName(field.Name)
				if !value.IsValid() || !value.CanSet() {
					continue
				}
				if record[i] == "" {
					continue
				}
				column := getJSONFieldName(field.Tag.Get("csv"))
				if column == "" {
					return nil, nil, fmt.Errorf("unknown column: %s", jsonTag)
				}
				originalColumn := column

				values := []string{}
				for _, str := range strings.Split(record[i], " ; ") {
					switch originalColumn {
					case "field_linked_agent":
						var c contributor.Contributor
						err := json.Unmarshal([]byte(str), &c)
						if err != nil {
							return nil, nil, fmt.Errorf("error unmarshalling contributor: %s %v", str, err)
						}
						str, err = resolver.resolveContributor(c)
						if err != nil {
							return nil, nil, fmt.Errorf("error resolving contributor: %s %v", str, err)
						}

					case "field_add_coverpage", "published":
						switch str {
						case "Yes":
							str = "1"
						case "No":
							str = "0"
						default:
							return nil, nil, fmt.Errorf("unknown %s: %s", jsonTag, str)
						}
					case "id", "parent_id":
						if !re.MatchString(str) {
							return nil, nil, fmt.Errorf("unknown %s: %s", jsonTag, str)
						}
					case "field_weight", "node_id":
						_, err := strconv.Atoi(str)
						if err != nil {
							return nil, nil, fmt.Errorf("unknown %s: %s", jsonTag, str)
						}
						str = strings.TrimLeft(str, "0")
					case "field_subject_hierarchical_geo":
						if _, ok := tgnCache[str]; ok {
							str = tgnCache[str]
							break
						}

						tgn, err := tgn.GetLocationFromTGN(str)
						if err != nil {
							return nil, nil, fmt.Errorf("unknown TGN: %s %v", str, err)
						}

						locationJSON, err := json.Marshal(tgn)
						if err != nil {
							return nil, nil, fmt.Errorf("error marshalling TGN: %s %v", str, err)
						}
						tgnCache[str] = string(locationJSON)
						str = tgnCache[str]

					case "field_rights":
						switch str {
						case "IN COPYRIGHT":
							str = "http://rightsstatements.org/vocab/InC/1.0/"
						case "IN COPYRIGHT - EU ORPHAN WORK":
							str = "http://rightsstatements.org/vocab/InC-OW-EU/1.0/"
						case "IN COPYRIGHT - EDUCATIONAL USE PERMITTED":
							str = "http://rightsstatements.org/vocab/InC-EDU/1.0/"
						case "IN COPYRIGHT - NON-COMMERCIAL USE PERMITTED":
							str = "http://rightsstatements.org/vocab/InC-NC/1.0/"
						case "IN COPYRIGHT - RIGHTS-HOLDER(S) UNLOCATABLE OR UNIDENTIFIABLE":
							str = "http://rightsstatements.org/vocab/InC-RUU/1.0/"
						case "NO COPYRIGHT - CONTRACTUAL RESTRICTIONS":
							str = "http://rightsstatements.org/vocab/NoC-CR/1.0/"
						case "NO COPYRIGHT - NON-COMMERCIAL USE ONLY":
							str = "http://rightsstatements.org/vocab/NoC-NC/1.0/"
						case "NO COPYRIGHT - OTHER KNOWN LEGAL RESTRICTIONS":
							str = "http://rightsstatements.org/vocab/NoC-OKLR/1.0/"
						case "NO COPYRIGHT - UNITED STATES":
							str = "http://rightsstatements.org/vocab/NoC-US/1.0/"
						case "COPYRIGHT NOT EVALUATED":
							str = "http://rightsstatements.org/vocab/CNE/1.0/"
						case "COPYRIGHT UNDETERMINED":
							str = "http://rightsstatements.org/vocab/UND/1.0/"
						case "NO KNOWN COPYRIGHT":
							str = "http://rightsstatements.org/vocab/NKC/1.0/"
						default:
							return nil, nil, fmt.Errorf("unknown %s: %s", jsonTag, str)
						}
					case "field_extent.attr0=page",
						"field_extent.attr0=dimensions",
						"field_extent.attr0=bytes",
						"field_extent.attr0=minutes",
						"field_abstract.attr0=description",
						"field_abstract.attr0=abstract",
						"field_note.attr0=preferred-citation",
						"field_note.attr0=capture-device",
						"field_note.attr0=ppi",
						"field_note.attr0=collection",
						"field_note.attr0=box",
						"field_note.attr0=series",
						"field_note.attr0=folder",
						"field_part_detail.attr0=volume",
						"field_part_detail.attr0=issue",
						"field_part_detail.attr0=page",
						"field_identifier.attr0=doi",
						"field_identifier.attr0=uri",
						"field_identifier.attr0=call-number",
						"field_identifier.attr0=report-number":
						components := strings.Split(originalColumn, ".attr0=")
						str = strings.ReplaceAll(str, `\`, `\\`)
						str = strings.ReplaceAll(str, `"`, `\"`)
						column = components[0]
						if column == "field_part_detail" {
							str = fmt.Sprintf(`{"number":"%s","type":"%s"}`, str, components[1])

						} else {
							str = fmt.Sprintf(`{"value":"%s","attr0":"%s"}`, str, components[1])
						}
					case "field_geographic_subject.vid=geographic_naf",
						"field_geographic_subject.vid=geographic_local":
						components := strings.Split(originalColumn, ".vid=")
						column = components[0]
						str = fmt.Sprintf("%s:%s", components[1], str)
					case "field_related_item.title":
						column = "field_related_item"
						str = fmt.Sprintf(`{"title": "%s"}`, str)
					case "field_related_item.identifier_type=issn":
						column = "field_related_item"
						str = fmt.Sprintf(`{"type": "issn", "identifier": "%s"}`, str)
					case "file", "supplemental_file":
						str = strings.ReplaceAll(str, `\`, `/`)
						if len(str) > 7 && str[0:6] == "/home/" {
							break
						}
						str = strings.TrimLeft(str, "/")
						if len(str) > 3 && str[0:3] != "mnt" {
							str = fmt.Sprintf("/mnt/islandora_staging/%s", str)
						}
					}

					str = strings.TrimSpace(str)
					values = append(values, str)
				}

				newHeaders[column] = true
				// replace the locally defined google sheets cell delimiter
				// with workbench's pipe delimiter
				row[column] = append(row[column], strings.Join(values, "|"))
			}
		}

		rows = append(rows, row)
	}

	return newHeaders, rows, nil
}

type drupalTermResolver struct {
	baseURL      string
	username     string
	password     string
	client       *http.Client
	peopleCache  map[string]int
	institutions map[string]int
}

func newDrupalTermResolver() *drupalTermResolver {
	baseURL := os.Getenv("FABRICATOR_TERM_LOOKUP_URL")
	if baseURL == "" {
		baseURL = "https://preserve.lehigh.edu"
	}
	username := os.Getenv("FABRICATOR_DRUPAL_USERNAME")
	if username == "" {
		username = "workbench"
	}
	password := os.Getenv("FABRICATOR_DRUPAL_PASSWORD")
	if password == "" {
		password = os.Getenv("ISLANDORA_WORKBENCH_PASSWORD")
	}

	return &drupalTermResolver{
		baseURL:      strings.TrimRight(baseURL, "/"),
		username:     username,
		password:     password,
		client:       http.DefaultClient,
		peopleCache:  map[string]int{},
		institutions: map[string]int{},
	}
}

func (d *drupalTermResolver) resolveContributor(c contributor.Contributor) (string, error) {
	parts := strings.Split(c.Name, ":")
	if len(parts) < 4 {
		return "", fmt.Errorf("poorly formatted contributor: %s", c.Name)
	}

	relator := strings.Join(parts[:2], ":")
	vocab := parts[2]
	name := strings.Join(parts[3:], ":")
	var (
		tid int
		err error
	)

	switch vocab {
	case "person":
		tid, err = d.ensurePerson(c, name)
	case "corporate_body":
		tid, err = d.ensureInstitution(name)
	default:
		return c.Name, nil
	}
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s:%d", relator, vocab, tid), nil
}

func (d *drupalTermResolver) ensurePerson(c contributor.Contributor, name string) (int, error) {
	cacheKey := strings.ToLower(strings.TrimSpace(strings.Join([]string{
		name,
		c.Email,
		c.Orcid,
		c.Institution,
	}, "|")))
	if tid, ok := d.peopleCache[cacheKey]; ok {
		return tid, nil
	}

	lookupParams := url.Values{}
	lookupParams.Set("name", name)
	lookupParams.Set("vocab", "person")

	var institutionID int
	if c.Email != "" {
		lookupParams.Set("email", c.Email)
	} else if c.Orcid != "" {
		lookupParams.Set("orcid", c.Orcid)
	} else if c.Institution != "" {
		var err error
		institutionID, err = d.ensureInstitution(c.Institution)
		if err != nil {
			return 0, err
		}
		lookupParams.Set("works_for", strconv.Itoa(institutionID))
	}

	tid, found, err := d.lookupTerm(lookupParams)
	if err != nil {
		return 0, err
	}
	if found {
		d.peopleCache[cacheKey] = tid
		return tid, nil
	}

	if institutionID == 0 && c.Institution != "" {
		institutionID, err = d.ensureInstitution(c.Institution)
		if err != nil {
			return 0, err
		}
	}

	tid, err = d.createTerm("person", name, c.Email, c.Orcid, institutionID)
	if err != nil {
		return 0, err
	}
	d.peopleCache[cacheKey] = tid
	return tid, nil
}

func (d *drupalTermResolver) ensureInstitution(name string) (int, error) {
	cacheKey := strings.ToLower(strings.TrimSpace(name))
	if tid, ok := d.institutions[cacheKey]; ok {
		return tid, nil
	}

	params := url.Values{}
	params.Set("name", name)
	params.Set("vocab", "corporate_body")

	tid, found, err := d.lookupTerm(params)
	if err != nil {
		return 0, err
	}
	if found {
		d.institutions[cacheKey] = tid
		return tid, nil
	}

	tid, err = d.createTerm("corporate_body", name, "", "", 0)
	if err != nil {
		return 0, err
	}
	d.institutions[cacheKey] = tid
	return tid, nil
}

func (d *drupalTermResolver) lookupTerm(params url.Values) (int, bool, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/term_from_term_name?%s", d.baseURL, params.Encode()), nil)
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return 0, false, fmt.Errorf("term lookup failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, false, err
	}
	tid, found, err := getTermIDFromBody(body)
	if err != nil {
		return 0, false, err
	}
	return tid, found, nil
}

func (d *drupalTermResolver) createTerm(vocab, name, email, orcid string, institutionID int) (int, error) {
	if d.password == "" {
		return 0, fmt.Errorf("unable to create term %q because FABRICATOR_DRUPAL_PASSWORD or ISLANDORA_WORKBENCH_PASSWORD is not set", name)
	}

	body := map[string]interface{}{
		"vid":  []map[string]string{{"target_id": vocab}},
		"name": []map[string]string{{"value": name}},
	}
	if email != "" {
		body["field_email"] = []map[string]string{{"value": email}}
	}
	if orcid != "" {
		body["field_identifier"] = []map[string]string{{"attr0": "orcid", "value": orcid}}
	}
	if institutionID > 0 {
		body["field_relationships"] = []map[string]interface{}{{
			"target_id": institutionID,
			"rel_type":  "schema:worksFor",
		}}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/entity/taxonomy_term?_format=json", d.baseURL), bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(d.username, d.password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		raw, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("term create failed with status %d: %s", resp.StatusCode, string(raw))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	tid, found, err := getTermIDFromBody(raw)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fmt.Errorf("term create response did not include a term id")
	}
	return tid, nil
}

type drupalTermResponse struct {
	Tid []struct {
		Value json.Number `json:"value"`
	} `json:"tid"`
}

func getTermIDFromBody(raw []byte) (int, bool, error) {
	var terms []drupalTermResponse
	if err := json.Unmarshal(raw, &terms); err == nil {
		return termIDFromResponseSlice(terms)
	}

	var term drupalTermResponse
	if err := json.Unmarshal(raw, &term); err != nil {
		return 0, false, err
	}
	return termIDFromResponse(term)
}

func termIDFromResponseSlice(terms []drupalTermResponse) (int, bool, error) {
	if len(terms) == 0 {
		return 0, false, nil
	}
	return termIDFromResponse(terms[0])
}

func termIDFromResponse(term drupalTermResponse) (int, bool, error) {
	if len(term.Tid) == 0 {
		return 0, false, nil
	}
	id, err := term.Tid[0].Value.Int64()
	if err != nil {
		return 0, false, err
	}
	return int(id), true, nil
}

package handlers

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
	headers, rows, linkedAgents, err := readCSVWithJSONTags(r)
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
	if len(linkedAgents) > 1 {
		csvFile := strings.Replace(target, ".csv", ".agents.csv", 1)
		files = append(files, csvFile)
		aFile, err := os.Create(csvFile)
		if err != nil {
			slog.Error("Failed to create file", "file", csvFile, "err", err)
			os.Exit(1)
		}

		aWriter := csv.NewWriter(aFile)
		for _, row := range linkedAgents {
			if err := aWriter.Write(row); err != nil {
				slog.Error("Failed to write record to CSV", "err", err)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}
		}
		aWriter.Flush()
		aFile.Close()
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

func readCSVWithJSONTags(r *http.Request) (map[string]bool, []map[string][]string, [][]string, error) {
	defer r.Body.Close()
	re := regexp.MustCompile(`^\d{1,4}$`)
	reader := csv.NewReader(r.Body)
	headers, err := reader.Read()
	if err != nil {
		return nil, nil, nil, err
	}

	var rows []map[string][]string
	newHeaders := map[string]bool{}

	var linkedAgents [][]string
	linkedAgents = append(linkedAgents, []string{
		"term_name",
		"field_contributor_status",
		"field_relationships",
		"field_email",
		"field_identifier",
	})
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
				if jsonTag == header {
					value := v.FieldByName(field.Name)
					if value.IsValid() && value.CanSet() {
						if record[i] == "" {
							continue
						}
						column := getJSONFieldName(field.Tag.Get("csv"))
						if column == "" {
							return nil, nil, nil, fmt.Errorf("unknown column: %s", jsonTag)
						}
						originalColumn := column

						values := []string{}
						for _, str := range strings.Split(record[i], " ; ") {
							switch originalColumn {
							case "field_linked_agent":
								var c contributor.Contributor
								err := json.Unmarshal([]byte(str), &c)
								if err != nil {
									return nil, nil, nil, fmt.Errorf("error unmarshalling contributor: %s %v", str, err)
								}

								str = c.Name
								if c.Institution != "" {
									str = fmt.Sprintf("%s - %s", str, c.Institution)
								}
								if c.Status != "" || c.Email != "" || c.Institution != "" || c.Orcid != "" {
									name := strings.Split(str, ":")
									if len(name) < 4 {
										return nil, nil, nil, fmt.Errorf("poorly formatted contributor: %s %v", str, err)
									}
									agent := []string{
										strings.Join(name[3:], ":"),
										c.Status,
										fmt.Sprintf("schema:worksFor:corporate_body:%s", c.Institution),
										c.Email,
										fmt.Sprintf(`{"attr0": "orcid", "value": "%s"}`, c.Orcid),
									}
									if c.Institution == "" {
										agent[2] = ""
									}
									if c.Orcid == "" {
										agent[4] = ""
									}

									linkedAgents = append(linkedAgents, agent)
								}

							case "field_add_coverpage", "published":
								switch str {
								case "Yes":
									str = "1"
								case "No":
									str = "0"
								default:
									return nil, nil, nil, fmt.Errorf("unknown %s: %s", jsonTag, str)
								}
							case "id", "parent_id":
								if !re.MatchString(str) {
									return nil, nil, nil, fmt.Errorf("unknown %s: %s", jsonTag, str)
								}
							case "field_weight", "node_id":
								_, err := strconv.Atoi(str)
								if err != nil {
									return nil, nil, nil, fmt.Errorf("unknown %s: %s", jsonTag, str)
								}
								str = strings.TrimLeft(str, "0")
							case "field_subject_hierarchical_geo":
								if _, ok := tgnCache[str]; ok {
									str = tgnCache[str]
									break
								}

								tgn, err := tgn.GetLocationFromTGN(str)
								if err != nil {
									return nil, nil, nil, fmt.Errorf("unknown TGN: %s %v", str, err)
								}

								locationJSON, err := json.Marshal(tgn)
								if err != nil {
									return nil, nil, nil, fmt.Errorf("error marshalling TGN: %s %v", str, err)
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
									return nil, nil, nil, fmt.Errorf("unknown %s: %s", jsonTag, str)
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
							case "file":
								str = strings.ReplaceAll(str, `\`, `/`)
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
			}
		}

		rows = append(rows, row)
	}

	return newHeaders, rows, linkedAgents, nil
}

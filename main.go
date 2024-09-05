package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/lehigh-university-libraries/fabricator/internal/contributor"
	"github.com/lehigh-university-libraries/fabricator/internal/handlers"
	"github.com/lehigh-university-libraries/fabricator/internal/tgn"
	"github.com/lehigh-university-libraries/go-islandora/workbench"
)

var linkedAgents [][]string

func getJSONFieldName(tag string) string {
	if commaIndex := strings.Index(tag, ","); commaIndex != -1 {
		return tag[:commaIndex]
	}
	return tag
}

func readCSVWithJSONTags(filePath string) (map[string]bool, []map[string][]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	re := regexp.MustCompile(`^\d{2,3}$`)
	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		return nil, nil, err
	}

	var rows []map[string][]string
	newHeaders := map[string]bool{}
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
							return nil, nil, fmt.Errorf("unknown column: %s", jsonTag)
						}

						values := []string{}
						for _, str := range strings.Split(record[i], " ; ") {
							switch column {
							case "field_linked_agent":
								var c contributor.Contributor
								err := json.Unmarshal([]byte(str), &c)
								if err != nil {
									return nil, nil, fmt.Errorf("error unmarshalling contributor: %s %v", str, err)
								}

								str = c.Name
								if c.Institution != "" {
									str = fmt.Sprintf("%s - %s", str, c.Institution)
								}
								if c.Status != "" || c.Email != "" || c.Institution != "" || c.Orcid != "" {
									name := strings.Split(str, ":")
									if len(name) < 4 {
										return nil, nil, fmt.Errorf("poorly formatted contributor: %s %v", str, err)
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
									return nil, nil, fmt.Errorf("unknown %s: %s", jsonTag, str)
								}
							case "id", "parent_id":
								if !re.MatchString(str) {
									return nil, nil, fmt.Errorf("unknown %s: %s", jsonTag, str)
								}
							case "field_weight":
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
								components := strings.Split(column, ".attr0=")
								column = components[0]
								if column == "field_part_detail" {
									str = fmt.Sprintf(`{"number":"%s","attr0":"%s"}`, str, components[1])

								} else {
									str = fmt.Sprintf(`{"value":"%s","attr0":"%s"}`, str, components[1])
								}
							case "field_geographic_subject.vid=geographic_naf",
								"field_geographic_subject.vid=geographic_local":
								components := strings.Split(column, ".vid=")
								column = components[0]
								str = fmt.Sprintf("%s:%s", components[1], str)
							case "field_related_item.title":
								column = "field_related_item"
								str = fmt.Sprintf(`{"title": "%s"}`, str)
							// TODO	case "field_related_item.identifier_type=issn":
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

	return newHeaders, rows, nil
}

func main() {
	var serverMode bool

	flag.BoolVar(&serverMode, "server", false, "Set to true to run as server")
	source := flag.String("source", "", "Path to the source CSV file")
	target := flag.String("target", "", "Path to the target CSV file")
	flag.Parse()

	if serverMode {
		// Start HTTP server
		http.HandleFunc("/workbench/check", handlers.CheckMyWork)

		slog.Info("Starting server on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			panic(err)
		}
	}

	// Define the source and target flags

	if *source == "" || *target == "" {
		fmt.Println("Source and target flags are required")
		flag.Usage()
		return
	}
	headers, rows, err := readCSVWithJSONTags(*source)
	if err != nil {
		slog.Error("Failed to read CSV", "err", err)
		os.Exit(1)
	}

	file, err := os.Create(*target)
	if err != nil {
		slog.Error("Failed to create file", "err", err)
		os.Exit(1)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	firstRow := make([]string, 0, len(headers))
	for header := range headers {
		firstRow = append(firstRow, header)
	}

	// finally, write the header to the CSV
	if err := writer.Write(firstRow); err != nil {
		slog.Error("Failed to write record to CSV", "err", err)
		os.Exit(1)
	}

	// write the rows to the CSV
	for _, row := range rows {
		record := []string{}
		for _, header := range firstRow {
			record = append(record, strings.Join(row[header], "|"))
		}
		if err := writer.Write(record); err != nil {
			slog.Error("Failed to write record to CSV", "err", err)
			os.Exit(1)
		}
	}
	slog.Info("CSV file has been written successfully")
	if len(linkedAgents) == 1 {
		return
	}

	csvFile := strings.Replace(*target, ".csv", ".agents.csv", 1)
	aFile, err := os.Create(csvFile)
	if err != nil {
		slog.Error("Failed to create file", "file", csvFile, "err", err)
		os.Exit(1)
	}
	defer aFile.Close()

	aWriter := csv.NewWriter(aFile)
	defer aWriter.Flush()

	for _, row := range linkedAgents {
		if err := aWriter.Write(row); err != nil {
			slog.Error("Failed to write record to CSV", "err", err)
			os.Exit(1)
		}
	}
	slog.Info("Linked Agent CSV file has been written successfully")
}

func StrInSlice(s string, sl []string) bool {
	for _, a := range sl {
		if a == s {
			return true
		}
	}
	return false
}

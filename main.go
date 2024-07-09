package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"

	"github.com/lehigh-university-libraries/go-islandora/workbench"
)

func getJSONFieldName(tag string) string {
	if commaIndex := strings.Index(tag, ","); commaIndex != -1 {
		return tag[:commaIndex]
	}
	return tag
}

func readCSVWithJSONTags(filePath string) ([]map[string][]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}

	var rows []map[string][]string
	newCsv := &workbench.SheetsCsv{}
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
						str := record[i]
						if str == "" {
							continue
						}
						column := getJSONFieldName(field.Tag.Get("csv"))
						switch column {
						case "field_linked_agent.vid":
							if str == "Corporate Body" {
								str = "corporate_body"
							} else if str == "Person" {
								str = "person"
							} else if str == "Family" {
								str = "family"
							} else {
								return nil, fmt.Errorf("unknown %s: %s", jsonTag, str)
							}
						case "field_linked_agent.rel_type":
							components := strings.Split(str, "|")
							str = components[0]
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
							str = fmt.Sprintf(`{"value":"%s","attr0":"%s"}`, str, components[1])
						case "field_geographic_subject.vid=geographic_naf",
							"field_geographic_subject.vid=geographic_local":
							components := strings.Split(column, ".vid=")
							column = components[0]
							str = fmt.Sprintf("%s:%s", components[1], str)
							/*
								case "field_related_item.title":
								case "field_related_item.identifier_type=issn":
								case "field_linked_agent.vid",
									"field_linked_agent.rel_type":
							*/
						case "file":
							str = strings.ReplaceAll(str, `\`, `/`)
							str = strings.TrimLeft(str, "/")
							if len(str) > 3 && str[0:3] != "mnt" {
								str = fmt.Sprintf("/mnt/islandora_staging/%s", str)
							}
						}

						str = strings.ReplaceAll(str, " ; ", "|")
						row[column] = append(row[column], str)
					}
				}
			}
		}

		rows = append(rows, row)
	}

	return rows, nil
}

func main() {
	// Define the source and target flags
	source := flag.String("source", "", "Path to the source CSV file")
	target := flag.String("target", "", "Path to the target CSV file")
	flag.Parse()

	if *source == "" || *target == "" {
		fmt.Println("Source and target flags are required")
		flag.Usage()
		return
	}
	rows, err := readCSVWithJSONTags(*source)
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

	// get all possible headers in the CSV
	headers := []string{}
	for header := range rows[0] {
		headers = append(headers, header)
	}

	// check any columns that have no values
	includeColumns := map[string]bool{}
	for k, row := range rows {
		for _, header := range headers {
			if header == "field_linked_agent.name" {
				name := rows[k][header]
				if len(name) == 0 {
					continue
				}
				header = "field_linked_agent"
				includeColumns[header] = true
				vid := rows[k]["field_linked_agent.vid"]
				rel := rows[k]["field_linked_agent.rel_type"]
				rows[k][header] = []string{
					fmt.Sprintf("%s:%s:%s", rel[0], vid[0], name[0]),
				}
			} else if header == "field_linked_agent.rel_type" || header == "field_linked_agent.vid" {
				continue
			} else if !includeColumns[header] && len(row[header]) > 0 {
				includeColumns[header] = true
			}
		}
	}

	// remove columns with no values from the header
	headers = []string{}
	for header, include := range includeColumns {
		if include {
			headers = append(headers, header)
		}
	}

	// finally, write the header to the CSV
	if err := writer.Write(headers); err != nil {
		slog.Error("Failed to write record to CSV", "err", err)
		os.Exit(1)
	}

	// write the rows to the CSV
	for _, row := range rows {
		record := []string{}
		for _, header := range headers {
			record = append(record, strings.Join(row[header], "|"))
		}
		if err := writer.Write(record); err != nil {
			slog.Error("Failed to write record to CSV", "err", err)
			os.Exit(1)
		}
	}

	slog.Info("CSV file has been written successfully")
}

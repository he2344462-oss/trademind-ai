package importcsv

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	MaxBytes      = 2 * 1024 * 1024
	MaxRows       = 5000
	MaxCellLength = 4096
)

type RowError struct {
	Row     int    `json:"row"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type Document struct {
	Headers []string
	Rows    []map[string]string
	Errors  []RowError
}

func Parse(input string) (Document, error) {
	if len(input) == 0 || len(input) > MaxBytes {
		return Document{}, fmt.Errorf("CSV must be between 1 byte and %d bytes", MaxBytes)
	}
	if !utf8.ValidString(input) {
		return Document{}, fmt.Errorf("CSV must use UTF-8 encoding")
	}
	input = strings.TrimPrefix(input, "\ufeff")
	r := csv.NewReader(strings.NewReader(input))
	r.FieldsPerRecord = -1
	headers, err := r.Read()
	if err != nil || len(headers) == 0 {
		return Document{}, fmt.Errorf("invalid CSV header")
	}
	seen := map[string]bool{}
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
		if headers[i] == "" || seen[headers[i]] || utf8.RuneCountInString(headers[i]) > 128 {
			return Document{}, fmt.Errorf("CSV headers must be non-empty and unique")
		}
		seen[headers[i]] = true
	}
	doc := Document{Headers: headers, Rows: []map[string]string{}, Errors: []RowError{}}
	for rowNumber := 2; ; rowNumber++ {
		record, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			doc.Errors = append(doc.Errors, RowError{Row: rowNumber, Message: "CSV row cannot be parsed"})
			continue
		}
		if len(doc.Rows) >= MaxRows {
			return Document{}, fmt.Errorf("CSV exceeds maximum of %d data rows", MaxRows)
		}
		row := map[string]string{}
		for i, header := range headers {
			value := ""
			if i < len(record) {
				value = strings.TrimSpace(record[i])
			}
			if utf8.RuneCountInString(value) > MaxCellLength {
				doc.Errors = append(doc.Errors, RowError{Row: rowNumber, Field: header, Message: "cell is too long"})
				continue
			}
			row[header] = value
		}
		doc.Rows = append(doc.Rows, row)
	}
	return doc, nil
}

func MapRow(row map[string]string, mapping map[string]string) map[string]string {
	out := map[string]string{}
	for source, target := range mapping {
		if strings.TrimSpace(target) != "" {
			out[strings.TrimSpace(target)] = strings.TrimSpace(row[source])
		}
	}
	return out
}

// UnsafeFormula detects spreadsheet formula prefixes in text fields. Numeric
// fields are parsed as numbers before this check and are therefore not affected.
func UnsafeFormula(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	switch value[0] {
	case '=', '+', '-', '@':
		return true
	}
	return false
}

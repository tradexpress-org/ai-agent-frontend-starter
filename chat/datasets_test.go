package chat

import (
	"reflect"
	"testing"
)

func TestParseJSONRecords(t *testing.T) {
	records, err := parseJSONRecords([]byte(`[{"name":"Ada","active":true},{"name":"Lin"}]`))
	if err != nil {
		t.Fatalf("parseJSONRecords returned an error: %v", err)
	}
	if len(records) != 2 || records[0]["name"] != "Ada" || records[0]["active"] != true {
		t.Fatalf("unexpected records: %#v", records)
	}
}

func TestParseJSONRecordsRejectsInvalidShapes(t *testing.T) {
	for _, input := range []string{
		`{"name":"Ada"}`,
		`[{"name":"Ada"},null]`,
		`null`,
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := parseJSONRecords([]byte(input)); err == nil {
				t.Fatalf("parseJSONRecords(%q) expected an error", input)
			}
		})
	}
}

func TestParseCSVRecords(t *testing.T) {
	records, err := parseCSVRecords([]byte("name,city\r\nAda,London\r\nLin,Beijing\r\n"))
	if err != nil {
		t.Fatalf("parseCSVRecords returned an error: %v", err)
	}
	want := []map[string]any{
		{"name": "Ada", "city": "London"},
		{"name": "Lin", "city": "Beijing"},
	}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("parseCSVRecords returned %#v, want %#v", records, want)
	}
}

func TestParseCSVRecordsRejectsInvalidHeadersAndRows(t *testing.T) {
	for _, input := range []string{
		"",
		"name,name\nAda,Smith\n",
		"name,city\nAda\n",
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := parseCSVRecords([]byte(input)); err == nil {
				t.Fatalf("parseCSVRecords(%q) expected an error", input)
			}
		})
	}
}

func TestParseDatasetFileUsesFilenameAndContentType(t *testing.T) {
	records, err := parseDatasetFile("upload", "application/json; charset=utf-8", []byte(`[{"id":1}]`))
	if err != nil {
		t.Fatalf("parseDatasetFile returned an error: %v", err)
	}
	if len(records) != 1 || records[0]["id"] != float64(1) {
		t.Fatalf("unexpected records: %#v", records)
	}

	if _, err := parseDatasetFile("upload.txt", "text/plain", []byte("id\n1\n")); err == nil {
		t.Fatal("parseDatasetFile expected an unsupported format error")
	}
}

func TestNormalizeDatasetName(t *testing.T) {
	name, err := normalizeDatasetName("  customers  ")
	if err != nil || name != "customers" {
		t.Fatalf("normalizeDatasetName returned %q, %v", name, err)
	}
	if _, err := normalizeDatasetName("   "); err == nil {
		t.Fatal("normalizeDatasetName expected an error for an empty name")
	}
}

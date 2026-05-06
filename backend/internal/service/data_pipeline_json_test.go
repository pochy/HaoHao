package service

import "testing"

func TestReadDataPipelineJSONMapsNestedArrayRecords(t *testing.T) {
	config := map[string]any{
		"recordPath":       "$",
		"includeRawRecord": true,
	}
	fields := []dataPipelineJSONField{
		{Column: "pokemon_id", Segments: []string{"id"}},
		{Column: "name_en", Segments: []string{"name", "english"}},
		{Column: "primary_type", Segments: []string{"type", "0"}},
		{Column: "types", Segments: []string{"type"}, Join: "|"},
		{Column: "sp_attack", Segments: []string{"base", "Sp. Attack"}},
		{Column: "ability_2_hidden", Segments: []string{"profile", "ability", "1", "1"}},
		{Column: "missing", Segments: []string{"missing"}, Default: "n/a"},
	}
	body := []byte(`[
		{
			"id": 1,
			"name": { "english": "Bulbasaur" },
			"type": ["Grass", "Poison"],
			"base": { "Sp. Attack": 65 },
			"profile": { "ability": [["Overgrow", "false"], ["Chlorophyll", "true"]] }
		}
	]`)

	rows, err := readDataPipelineJSON(body, config, fields, 100)
	if err != nil {
		t.Fatalf("readDataPipelineJSON() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	row := rows[0]
	assertJSONColumn(t, row, "pokemon_id", "1")
	assertJSONColumn(t, row, "name_en", "Bulbasaur")
	assertJSONColumn(t, row, "primary_type", "Grass")
	assertJSONColumn(t, row, "types", "Grass|Poison")
	assertJSONColumn(t, row, "sp_attack", "65")
	assertJSONColumn(t, row, "ability_2_hidden", "true")
	assertJSONColumn(t, row, "missing", "n/a")
	assertJSONColumn(t, row, "row_number", "1")
	assertJSONColumn(t, row, "record_path", "$[0]")
	if row["raw_record_json"] == "" {
		t.Fatal("raw_record_json is empty")
	}
}

func TestDataPipelineJSONFieldsRejectsDuplicateColumns(t *testing.T) {
	_, err := dataPipelineJSONFields(map[string]any{
		"fields": []any{
			map[string]any{"column": "name", "pathSegments": []any{"name", "english"}},
			map[string]any{"column": "name", "pathSegments": []any{"name", "japanese"}},
		},
	})
	if err == nil {
		t.Fatal("dataPipelineJSONFields() error = nil, want duplicate column error")
	}
}

func TestDataPipelineJSONFieldsAllowsRawRecordOnlyConfig(t *testing.T) {
	fields, err := dataPipelineJSONFields(map[string]any{"fields": []any{}})
	if err != nil {
		t.Fatalf("dataPipelineJSONFields() error = %v", err)
	}
	if len(fields) != 0 {
		t.Fatalf("field count = %d, want 0", len(fields))
	}
}

func TestExtractDataPipelineJSONRecordsUsesStepMetadataColumns(t *testing.T) {
	config := map[string]any{
		"recordPath":       "items",
		"includeRawRecord": true,
	}
	fields := []dataPipelineJSONField{
		{Column: "sku", Segments: []string{"sku"}},
		{Column: "tags", Segments: []string{"tags"}, Join: ","},
	}
	root := map[string]any{
		"items": []any{
			map[string]any{"sku": "potion", "tags": []any{"medicine", "shop"}},
		},
	}

	rows, err := extractDataPipelineJSONRecords(root, config, fields, 100, "json_row_number", "json_record_path")
	if err != nil {
		t.Fatalf("extractDataPipelineJSONRecords() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	row := rows[0]
	assertJSONColumn(t, row, "json_row_number", "1")
	assertJSONColumn(t, row, "json_record_path", "$.items[0]")
	assertJSONColumn(t, row, "sku", "potion")
	assertJSONColumn(t, row, "tags", "medicine,shop")
	if row["raw_record_json"] == "" {
		t.Fatal("raw_record_json is empty")
	}
}

func assertJSONColumn(t *testing.T, row map[string]any, column, want string) {
	t.Helper()
	got := row[column]
	if got != want {
		t.Fatalf("%s = %q, want %q", column, got, want)
	}
}

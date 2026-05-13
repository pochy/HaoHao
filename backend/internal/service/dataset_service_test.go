package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"example.com/haohao/backend/internal/db"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type fakeDatasetColumnType struct {
	name     string
	scanType reflect.Type
}

func (f fakeDatasetColumnType) Name() string             { return f.name }
func (f fakeDatasetColumnType) Nullable() bool           { return f.scanType.Kind() == reflect.Ptr }
func (f fakeDatasetColumnType) ScanType() reflect.Type   { return f.scanType }
func (f fakeDatasetColumnType) DatabaseTypeName() string { return f.scanType.String() }

type fakeDatasetRows struct {
	columns []string
	types   []driver.ColumnType
	values  [][]any
	index   int
	err     error
}

func (f *fakeDatasetRows) Next() bool {
	if f.index < 0 {
		f.index = 0
	} else {
		f.index++
	}
	return f.index < len(f.values)
}

func (f *fakeDatasetRows) Scan(dest ...any) error {
	if f.index < 0 || f.index >= len(f.values) {
		return io.EOF
	}
	row := f.values[f.index]
	for i := range dest {
		if i >= len(row) {
			continue
		}
		assignFakeScanValue(dest[i], row[i])
	}
	return nil
}

func (f *fakeDatasetRows) ScanStruct(dest any) error        { return nil }
func (f *fakeDatasetRows) ColumnTypes() []driver.ColumnType { return f.types }
func (f *fakeDatasetRows) Totals(dest ...any) error         { return nil }
func (f *fakeDatasetRows) Columns() []string                { return f.columns }
func (f *fakeDatasetRows) Close() error                     { return nil }
func (f *fakeDatasetRows) Err() error                       { return f.err }
func (f *fakeDatasetRows) HasData() bool                    { return len(f.values) > 0 }

func assignFakeScanValue(dest any, value any) {
	target := reflect.ValueOf(dest)
	if !target.IsValid() || target.Kind() != reflect.Ptr || target.IsNil() {
		return
	}
	elem := target.Elem()
	if value == nil {
		elem.Set(reflect.Zero(elem.Type()))
		return
	}
	source := reflect.ValueOf(value)
	if elem.Kind() == reflect.Ptr {
		ptr := reflect.New(elem.Type().Elem())
		if source.Type().AssignableTo(elem.Type().Elem()) {
			ptr.Elem().Set(source)
			elem.Set(ptr)
		}
		return
	}
	if source.Type().AssignableTo(elem.Type()) {
		elem.Set(source)
	}
}

type fakeDatasetClickHouseConn struct {
	rows  driver.Rows
	query string
}

func (f *fakeDatasetClickHouseConn) Contributors() []string { return nil }
func (f *fakeDatasetClickHouseConn) ServerVersion() (*driver.ServerVersion, error) {
	return nil, nil
}
func (f *fakeDatasetClickHouseConn) Select(context.Context, any, string, ...any) error { return nil }
func (f *fakeDatasetClickHouseConn) Query(_ context.Context, query string, _ ...any) (driver.Rows, error) {
	f.query = query
	return f.rows, nil
}
func (f *fakeDatasetClickHouseConn) QueryRow(context.Context, string, ...any) driver.Row { return nil }
func (f *fakeDatasetClickHouseConn) PrepareBatch(context.Context, string, ...driver.PrepareBatchOption) (driver.Batch, error) {
	return nil, nil
}
func (f *fakeDatasetClickHouseConn) Exec(context.Context, string, ...any) error { return nil }
func (f *fakeDatasetClickHouseConn) AsyncInsert(context.Context, string, bool, ...any) error {
	return nil
}
func (f *fakeDatasetClickHouseConn) Ping(context.Context) error { return nil }
func (f *fakeDatasetClickHouseConn) Stats() driver.Stats        { return driver.Stats{} }
func (f *fakeDatasetClickHouseConn) Close() error               { return nil }

func TestDatasetClickHouseSettingsEnableExternalSpill(t *testing.T) {
	service := &DatasetService{chConfig: DatasetClickHouseConfig{
		QueryMaxSeconds:     60,
		QueryMaxMemoryBytes: 1024 * 1024 * 1024,
		QueryMaxRowsToRead:  100000000,
		QueryMaxThreads:     4,
	}}

	for name, settings := range map[string]map[string]any{
		"query":  service.querySettings(),
		"export": service.exportQuerySettings(),
	} {
		t.Run(name, func(t *testing.T) {
			want := int64(512 * 1024 * 1024)
			if got := settings["max_bytes_before_external_sort"]; got != want {
				t.Fatalf("max_bytes_before_external_sort = %v, want %v", got, want)
			}
			if got := settings["max_bytes_before_external_group_by"]; got != want {
				t.Fatalf("max_bytes_before_external_group_by = %v, want %v", got, want)
			}
		})
	}
}

func TestHydrateWorkTableColumns(t *testing.T) {
	conn := &fakeDatasetClickHouseConn{
		rows: &fakeDatasetRows{
			index: -1,
			values: [][]any{
				{"hh_t_1_work", "first_table", uint64(1), "id", "String"},
				{"hh_t_1_work", "first_table", uint64(2), "amount", "Float64"},
				{"hh_t_1_work", "second_table", uint64(1), "status", "Nullable(String)"},
			},
		},
	}
	service := &DatasetService{clickhouse: conn}
	items := []DatasetWorkTable{
		{Database: "hh_t_1_work", Table: "first_table"},
		{Database: "hh_t_1_work", Table: "second_table"},
	}

	if err := service.hydrateWorkTableColumns(context.Background(), items); err != nil {
		t.Fatalf("hydrateWorkTableColumns() error = %v", err)
	}
	if !strings.Contains(conn.query, "system.columns") {
		t.Fatalf("hydrateWorkTableColumns() query = %q, want system.columns", conn.query)
	}
	if got, want := len(items[0].Columns), 2; got != want {
		t.Fatalf("first table column count = %d, want %d", got, want)
	}
	if got, want := items[0].Columns[1].ColumnName, "amount"; got != want {
		t.Fatalf("first table second column = %q, want %q", got, want)
	}
	if got, want := items[1].Columns[0].ClickHouseType, "Nullable(String)"; got != want {
		t.Fatalf("second table column type = %q, want %q", got, want)
	}
}

func TestSanitizeDatasetColumns(t *testing.T) {
	got := sanitizeDatasetColumns([]string{
		"Customer ID",
		"Customer ID",
		"123 Total",
		"価格",
		"",
		"a_2",
		"a_2",
	})
	want := []string{
		"customer_id",
		"customer_id_2",
		"c_123_total",
		"column_4",
		"column_5",
		"a_2",
		"a_2_2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sanitizeDatasetColumns() = %#v, want %#v", got, want)
	}
}

func TestValidateDatasetSQLAllowsTenantScopedStatements(t *testing.T) {
	cases := []string{
		"SELECT * FROM hh_t_7_raw.ds_abc LIMIT 10",
		"SELECT * FROM hh_t_7_gold.gm_sales LIMIT 10",
		"CREATE TABLE hh_t_7_work.joined AS SELECT * FROM hh_t_7_raw.ds_abc",
		"INSERT INTO hh_t_7_work.joined SELECT * FROM hh_t_7_raw.ds_abc",
		"SELECT ';' AS semicolon",
	}
	for _, statement := range cases {
		if _, err := validateDatasetSQL(7, statement); err != nil {
			t.Fatalf("validateDatasetSQL(%q) returned error: %v", statement, err)
		}
	}
}

func TestValidateDatasetSQLRejectsUnsafeStatements(t *testing.T) {
	cases := []string{
		"SELECT * FROM hh_t_8_raw.ds_abc",
		"SELECT * FROM hh_t_8_gold.gm_sales",
		"SELECT * FROM hh_t_7_gold_internal.gp_run",
		"SELECT * FROM system.tables",
		"SELECT * FROM `system`.`tables`",
		"SELECT * FROM default.some_table",
		"SELECT * FROM file('/tmp/source.csv')",
		"SELECT * FROM `file`('/tmp/source.csv')",
		"SELECT * FROM url('https://example.test/a.csv')",
		"SELECT 1; SELECT 2",
	}
	for _, statement := range cases {
		_, err := validateDatasetSQL(7, statement)
		if !errors.Is(err, ErrUnsafeDatasetSQL) {
			t.Fatalf("validateDatasetSQL(%q) error = %v, want ErrUnsafeDatasetSQL", statement, err)
		}
	}
}

func TestValidateDatasetSQLRejectsEmptyStatements(t *testing.T) {
	_, err := validateDatasetSQL(7, " ; ")
	if !errors.Is(err, ErrInvalidDatasetInput) {
		t.Fatalf("validateDatasetSQL(empty) error = %v, want ErrInvalidDatasetInput", err)
	}
}

func TestValidateDatasetWorkTableRef(t *testing.T) {
	database, table, err := validateDatasetWorkTableRef(7, "hh_t_7_work", "hai_category_summary")
	if err != nil {
		t.Fatalf("validateDatasetWorkTableRef(valid) error = %v", err)
	}
	if database != "hh_t_7_work" || table != "hai_category_summary" {
		t.Fatalf("validateDatasetWorkTableRef(valid) = %q, %q", database, table)
	}

	if _, _, err := validateDatasetWorkTableRef(7, "hh_t_8_work", "hai_category_summary"); !errors.Is(err, ErrDatasetWorkTableNotFound) {
		t.Fatalf("validateDatasetWorkTableRef(cross tenant) error = %v, want ErrDatasetWorkTableNotFound", err)
	}
	if _, _, err := validateDatasetWorkTableRef(7, "hh_t_7_raw", "ds_abc"); !errors.Is(err, ErrDatasetWorkTableNotFound) {
		t.Fatalf("validateDatasetWorkTableRef(raw db) error = %v, want ErrDatasetWorkTableNotFound", err)
	}
	if _, _, err := validateDatasetWorkTableRef(7, "hh_t_7_work", ""); !errors.Is(err, ErrInvalidDatasetInput) {
		t.Fatalf("validateDatasetWorkTableRef(empty table) error = %v, want ErrInvalidDatasetInput", err)
	}
	if _, _, err := validateDatasetWorkTableRef(7, "hh_t_7_work", "bad\nname"); !errors.Is(err, ErrInvalidDatasetInput) {
		t.Fatalf("validateDatasetWorkTableRef(control rune) error = %v, want ErrInvalidDatasetInput", err)
	}
}

func TestParseDatasetCreateTableRefs(t *testing.T) {
	cases := []struct {
		name      string
		statement string
		want      []datasetWorkTableRef
	}{
		{
			name:      "qualified quoted work table",
			statement: "CREATE TABLE IF NOT EXISTS `hh_t_7_work`.`hai_category_summary` AS SELECT 1",
			want:      []datasetWorkTableRef{{Database: "hh_t_7_work", Table: "hai_category_summary"}},
		},
		{
			name:      "default work database",
			statement: "CREATE OR REPLACE TABLE joined AS SELECT 1",
			want:      []datasetWorkTableRef{{Database: "hh_t_7_work", Table: "joined"}},
		},
		{
			name:      "ignores raw table",
			statement: "CREATE TABLE hh_t_7_raw.ds_copy AS SELECT 1",
			want:      []datasetWorkTableRef{},
		},
		{
			name:      "ignores string literals",
			statement: "SELECT 'CREATE TABLE hh_t_7_work.not_real'",
			want:      []datasetWorkTableRef{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseDatasetCreateTableRefs(7, tc.statement)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseDatasetCreateTableRefs() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestHasMultipleDatasetStatements(t *testing.T) {
	cases := []struct {
		statement string
		want      bool
	}{
		{statement: "SELECT 1", want: false},
		{statement: "SELECT 1;", want: false},
		{statement: "SELECT ';' AS value", want: false},
		{statement: "SELECT `semi;colon` FROM t", want: false},
		{statement: "SELECT 1; SELECT 2", want: true},
	}
	for _, tc := range cases {
		if got := hasMultipleDatasetStatements(tc.statement); got != tc.want {
			t.Fatalf("hasMultipleDatasetStatements(%q) = %v, want %v", tc.statement, got, tc.want)
		}
	}
}

func TestDatasetInsertSQLUsesNativeBatchForm(t *testing.T) {
	statement := datasetInsertSQL(db.Dataset{RawDatabase: "hh_t_1_raw", RawTable: "ds_test"})
	if strings.Contains(strings.ToUpper(statement), "VALUES") {
		t.Fatalf("datasetInsertSQL() = %q, must not include VALUES for clickhouse native batch", statement)
	}
	if statement != "INSERT INTO `hh_t_1_raw`.`ds_test`" {
		t.Fatalf("datasetInsertSQL() = %q", statement)
	}
}

func TestDatasetSourceAllowsDriveCSVFiles(t *testing.T) {
	if !datasetSourcePurposeAllowed("drive") {
		t.Fatal("datasetSourcePurposeAllowed(drive) = false")
	}
	if !datasetSourcePurposeAllowed(DatasetSourceFilePurpose) {
		t.Fatal("datasetSourcePurposeAllowed(dataset_source) = false")
	}
	if datasetSourcePurposeAllowed("attachment") {
		t.Fatal("datasetSourcePurposeAllowed(attachment) = true")
	}
	if !isDatasetCSVSource("customers.csv", "application/octet-stream") {
		t.Fatal("isDatasetCSVSource(.csv) = false")
	}
	if !isDatasetCSVSource("customers", "application/vnd.ms-excel") {
		t.Fatal("isDatasetCSVSource(application/vnd.ms-excel) = false")
	}
	if isDatasetCSVSource("customers.json", "application/json") {
		t.Fatal("isDatasetCSVSource(json) = true")
	}
}

func TestDriveEffectiveUploadMaxBytesHonorsDatasetOverride(t *testing.T) {
	const fileMax = int64(100)
	const datasetMax = int64(1000)
	const policyMax = int64(50)

	if got := driveEffectiveUploadMaxBytes(fileMax, 0, policyMax); got != policyMax {
		t.Fatalf("driveEffectiveUploadMaxBytes(no override) = %d, want %d", got, policyMax)
	}
	if got := driveEffectiveUploadMaxBytes(fileMax, datasetMax, policyMax); got != datasetMax {
		t.Fatalf("driveEffectiveUploadMaxBytes(dataset override) = %d, want %d", got, datasetMax)
	}
}

func TestDatasetScanDestinationsUsesConcreteTypes(t *testing.T) {
	columnTypes := []driver.ColumnType{
		fakeDatasetColumnType{name: "__row_number", scanType: reflect.TypeOf(uint64(0))},
		fakeDatasetColumnType{name: "name", scanType: reflect.TypeOf((*string)(nil))},
	}
	holders, dest := datasetScanDestinations(columnTypes, 2)
	if _, ok := dest[0].(*uint64); !ok {
		t.Fatalf("dest[0] = %T, want *uint64", dest[0])
	}
	if _, ok := dest[1].(**string); !ok {
		t.Fatalf("dest[1] = %T, want **string", dest[1])
	}

	*dest[0].(*uint64) = 42
	value := "alice"
	*dest[1].(**string) = &value
	if got := datasetScannedValue(holders[0]); got != uint64(42) {
		t.Fatalf("datasetScannedValue(uint64) = %#v", got)
	}
	if got := datasetScannedValue(holders[1]); got != "alice" {
		t.Fatalf("datasetScannedValue(nullable string) = %#v", got)
	}

	*dest[1].(**string) = nil
	if got := datasetScannedValue(holders[1]); got != nil {
		t.Fatalf("datasetScannedValue(nil nullable string) = %#v", got)
	}
}

func TestScanDatasetRowsPageUsesRowNumberCursor(t *testing.T) {
	rows := &fakeDatasetRows{
		index:   -1,
		columns: []string{"__row_number", "name", "age"},
		types: []driver.ColumnType{
			fakeDatasetColumnType{name: "__row_number", scanType: reflect.TypeOf(uint64(0))},
			fakeDatasetColumnType{name: "name", scanType: reflect.TypeOf((*string)(nil))},
			fakeDatasetColumnType{name: "age", scanType: reflect.TypeOf((*string)(nil))},
		},
		values: [][]any{
			{uint64(1), "alice", "10"},
			{uint64(2), "bob", "20"},
			{uint64(3), "carol", "30"},
		},
	}
	columns, items, nextCursor, hasMore, err := scanDatasetRowsPage(rows, 2)
	if err != nil {
		t.Fatalf("scanDatasetRowsPage() error = %v", err)
	}
	if !reflect.DeepEqual(columns, []string{"name", "age"}) {
		t.Fatalf("scanDatasetRowsPage() columns = %#v", columns)
	}
	if len(items) != 2 {
		t.Fatalf("scanDatasetRowsPage() items length = %d, want 2", len(items))
	}
	if items[0]["name"] != "alice" || items[1]["name"] != "bob" {
		t.Fatalf("scanDatasetRowsPage() items = %#v", items)
	}
	if !hasMore {
		t.Fatal("scanDatasetRowsPage() hasMore = false, want true")
	}
	if nextCursor == nil || *nextCursor != 2 {
		t.Fatalf("scanDatasetRowsPage() nextCursor = %#v, want 2", nextCursor)
	}
}

func TestDatasetWorkTableExportFormatNormalizationAndSpec(t *testing.T) {
	cases := []struct {
		input string
		want  string
		ext   string
		ct    string
	}{
		{input: "", want: "csv", ext: ".csv", ct: "text/csv"},
		{input: " JSON ", want: "json", ext: ".ndjson", ct: "application/x-ndjson"},
		{input: "parquet", want: "parquet", ext: ".parquet", ct: "application/vnd.apache.parquet"},
	}
	for _, tc := range cases {
		got, err := normalizeDatasetWorkTableExportFormat(tc.input)
		if err != nil {
			t.Fatalf("normalizeDatasetWorkTableExportFormat(%q) error = %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("normalizeDatasetWorkTableExportFormat(%q) = %q, want %q", tc.input, got, tc.want)
		}
		spec, ok := datasetWorkTableExportFormatSpecFor(got)
		if !ok {
			t.Fatalf("datasetWorkTableExportFormatSpecFor(%q) not found", got)
		}
		if spec.Extension != tc.ext || spec.ContentType != tc.ct {
			t.Fatalf("spec(%q) = %#v, want ext=%q contentType=%q", got, spec, tc.ext, tc.ct)
		}
	}
	if _, err := normalizeDatasetWorkTableExportFormat("xml"); !errors.Is(err, ErrInvalidDatasetInput) {
		t.Fatalf("normalizeDatasetWorkTableExportFormat(unsupported) error = %v, want ErrInvalidDatasetInput", err)
	}
}

func TestNormalizeDatasetSyncMode(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "", want: "full_refresh"},
		{input: " full_REFRESH ", want: "full_refresh"},
	}
	for _, tc := range cases {
		got, err := normalizeDatasetSyncMode(tc.input)
		if err != nil {
			t.Fatalf("normalizeDatasetSyncMode(%q) error = %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("normalizeDatasetSyncMode(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
	if _, err := normalizeDatasetSyncMode("append"); !errors.Is(err, ErrInvalidDatasetInput) {
		t.Fatalf("normalizeDatasetSyncMode(append) error = %v, want ErrInvalidDatasetInput", err)
	}
}

func TestWorkTableExportScheduleNextRun(t *testing.T) {
	now := time.Date(2026, 5, 2, 2, 30, 0, 0, time.FixedZone("JST", 9*60*60))
	weekday := int32(6)
	monthDay := int32(2)
	cases := []struct {
		name      string
		frequency string
		runTime   string
		weekday   *int32
		monthDay  *int32
		want      time.Time
	}{
		{
			name:      "daily same day",
			frequency: datasetWorkTableExportFrequencyDaily,
			runTime:   "03:00",
			want:      time.Date(2026, 5, 1, 18, 0, 0, 0, time.UTC),
		},
		{
			name:      "weekly rolls forward",
			frequency: datasetWorkTableExportFrequencyWeekly,
			runTime:   "01:00",
			weekday:   &weekday,
			want:      time.Date(2026, 5, 8, 16, 0, 0, 0, time.UTC),
		},
		{
			name:      "monthly rolls forward",
			frequency: datasetWorkTableExportFrequencyMonthly,
			runTime:   "02:00",
			monthDay:  &monthDay,
			want:      time.Date(2026, 6, 1, 17, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := nextWorkTableExportScheduleRunAfter(tc.frequency, "Asia/Tokyo", tc.runTime, tc.weekday, tc.monthDay, now)
			if err != nil {
				t.Fatalf("nextWorkTableExportScheduleRunAfter() error = %v", err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("nextWorkTableExportScheduleRunAfter() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestNormalizeWorkTableExportScheduleInputValidation(t *testing.T) {
	weekday := int32(1)
	input := DatasetWorkTableExportScheduleInput{
		Format:        "json",
		Frequency:     "weekly",
		Timezone:      "Asia/Tokyo",
		RunTime:       "03:00",
		Weekday:       &weekday,
		RetentionDays: 14,
	}
	got, _, err := normalizeWorkTableExportScheduleInput(input, time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatalf("normalizeWorkTableExportScheduleInput() error = %v", err)
	}
	if got.Format != "json" || got.Frequency != "weekly" || got.RetentionDays != 14 || got.Weekday == nil || *got.Weekday != 1 {
		t.Fatalf("normalizeWorkTableExportScheduleInput() = %#v", got)
	}
	if _, _, err := normalizeWorkTableExportScheduleInput(DatasetWorkTableExportScheduleInput{Frequency: "weekly", Timezone: "Asia/Tokyo", RunTime: "03:00"}, time.Now(), nil); !errors.Is(err, ErrInvalidDatasetInput) {
		t.Fatalf("weekly without weekday error = %v, want ErrInvalidDatasetInput", err)
	}
	if _, _, err := normalizeWorkTableExportScheduleInput(DatasetWorkTableExportScheduleInput{Timezone: "Not/AZone"}, time.Now(), nil); !errors.Is(err, ErrInvalidDatasetInput) {
		t.Fatalf("invalid timezone error = %v, want ErrInvalidDatasetInput", err)
	}
}

func TestDatasetClickHouseTypeUnsupportedForParquet(t *testing.T) {
	cases := []struct {
		chType string
		want   bool
	}{
		{chType: "String", want: false},
		{chType: "Nullable(DateTime64(3))", want: false},
		{chType: "LowCardinality(Nullable(String))", want: false},
		{chType: "Array(String)", want: true},
		{chType: "Nullable(Array(String))", want: true},
		{chType: "Map(String, String)", want: true},
		{chType: "Tuple(String, UInt64)", want: true},
		{chType: "Nested(name String)", want: true},
	}
	for _, tc := range cases {
		if got := datasetClickHouseTypeUnsupportedForParquet(tc.chType); got != tc.want {
			t.Fatalf("datasetClickHouseTypeUnsupportedForParquet(%q) = %v, want %v", tc.chType, got, tc.want)
		}
	}
}

func TestWriteDatasetRowsJSONLines(t *testing.T) {
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 600, time.UTC)
	rows := &fakeDatasetRows{
		index:   -1,
		columns: []string{"id", "name", "created_at"},
		types: []driver.ColumnType{
			fakeDatasetColumnType{name: "id", scanType: reflect.TypeOf(uint64(0))},
			fakeDatasetColumnType{name: "name", scanType: reflect.TypeOf((*string)(nil))},
			fakeDatasetColumnType{name: "created_at", scanType: reflect.TypeOf(time.Time{})},
		},
		values: [][]any{
			{uint64(1), "alice", createdAt},
			{uint64(2), nil, createdAt},
		},
	}
	var buf bytes.Buffer
	if err := writeDatasetRowsJSONLines(rows, &buf); err != nil {
		t.Fatalf("writeDatasetRowsJSONLines() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("JSON lines count = %d, want 2; body=%q", len(lines), buf.String())
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unmarshal first line: %v", err)
	}
	if first["id"] != float64(1) || first["name"] != "alice" || first["created_at"] != "2026-01-02T03:04:05.0000006Z" {
		t.Fatalf("first line = %#v", first)
	}
	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("unmarshal second line: %v", err)
	}
	if second["id"] != float64(2) || second["name"] != nil {
		t.Fatalf("second line = %#v", second)
	}
}

func TestCopyWorkTableParquetUsesClickHouseHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "default" || password != "secret" {
			t.Fatalf("BasicAuth = %q/%q/%v", username, password, ok)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if got, want := string(body), "SELECT * FROM `hh_t_1_work`.`sales` FORMAT Parquet"; got != want {
			t.Fatalf("query body = %q, want %q", got, want)
		}
		if got := r.URL.Query().Get("max_execution_time"); got != "11" {
			t.Fatalf("max_execution_time = %q, want 11", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("PAR1fake"))
	}))
	defer server.Close()

	service := &DatasetService{chConfig: DatasetClickHouseConfig{
		HTTPURL:             server.URL,
		Database:            "default",
		Username:            "default",
		Password:            "secret",
		QueryMaxSeconds:     11,
		QueryMaxMemoryBytes: 1024,
		QueryMaxRowsToRead:  2048,
		QueryMaxThreads:     2,
	}}
	var out bytes.Buffer
	if err := service.copyWorkTableParquet(context.Background(), "hh_t_1_work", "sales", &out); err != nil {
		t.Fatalf("copyWorkTableParquet() error = %v", err)
	}
	if got := out.String(); got != "PAR1fake" {
		t.Fatalf("copyWorkTableParquet body = %q", got)
	}
}

func TestCopyWorkTableParquetReturnsHTTPErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	service := &DatasetService{chConfig: DatasetClickHouseConfig{HTTPURL: server.URL, Username: "default"}}
	var out bytes.Buffer
	err := service.copyWorkTableParquet(context.Background(), "hh_t_1_work", "sales", &out)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("copyWorkTableParquet() error = %v, want body", err)
	}
}

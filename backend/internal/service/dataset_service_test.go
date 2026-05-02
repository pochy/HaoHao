package service

import (
	"errors"
	"reflect"
	"strings"
	"testing"

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

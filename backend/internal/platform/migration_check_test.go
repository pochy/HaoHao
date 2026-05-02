package platform

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMaxMigrationVersion(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"0001_initial.up.sql",
		"0002_initial.down.sql",
		"0027_dataset_query_job_dataset_scope.up.sql",
		"README.md",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- test\n"), 0o600); err != nil {
			t.Fatalf("write migration file: %v", err)
		}
	}

	got, err := MaxMigrationVersion(dir)
	if err != nil {
		t.Fatalf("MaxMigrationVersion returned error: %v", err)
	}
	if got != 27 {
		t.Fatalf("MaxMigrationVersion = %d, want 27", got)
	}
}

func TestEvaluateMigrationCheck(t *testing.T) {
	readErr := errors.New("schema_migrations does not exist")
	tests := []struct {
		name         string
		mode         MigrationCheckMode
		current      int64
		expected     int64
		dirty        bool
		err          error
		wantStatus   string
		wantNeedsLog bool
		wantFails    bool
	}{
		{
			name:       "up to date",
			mode:       MigrationCheckModeWarn,
			current:    27,
			expected:   27,
			wantStatus: "up_to_date",
		},
		{
			name:         "behind warn",
			mode:         MigrationCheckModeWarn,
			current:      26,
			expected:     27,
			wantStatus:   "behind",
			wantNeedsLog: true,
		},
		{
			name:         "dirty fail",
			mode:         MigrationCheckModeFail,
			current:      27,
			expected:     27,
			dirty:        true,
			wantStatus:   "dirty",
			wantNeedsLog: true,
			wantFails:    true,
		},
		{
			name:       "off",
			mode:       MigrationCheckModeOff,
			err:        readErr,
			wantStatus: "off",
		},
		{
			name:         "unreadable fail",
			mode:         MigrationCheckModeFail,
			err:          readErr,
			wantStatus:   "schema_migrations_unreadable",
			wantNeedsLog: true,
			wantFails:    true,
		},
		{
			name:         "ahead warn",
			mode:         MigrationCheckModeWarn,
			current:      28,
			expected:     27,
			wantStatus:   "ahead",
			wantNeedsLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EvaluateMigrationCheck(tt.mode, tt.current, tt.expected, tt.dirty, tt.err)
			if result.Status != tt.wantStatus {
				t.Fatalf("Status = %q, want %q", result.Status, tt.wantStatus)
			}
			if result.NeedsLog() != tt.wantNeedsLog {
				t.Fatalf("NeedsLog = %t, want %t", result.NeedsLog(), tt.wantNeedsLog)
			}
			if result.FailsStartup() != tt.wantFails {
				t.Fatalf("FailsStartup = %t, want %t", result.FailsStartup(), tt.wantFails)
			}
		})
	}
}

func TestNormalizeMigrationCheckMode(t *testing.T) {
	mode, err := NormalizeMigrationCheckMode("")
	if err != nil {
		t.Fatalf("NormalizeMigrationCheckMode returned error: %v", err)
	}
	if mode != MigrationCheckModeWarn {
		t.Fatalf("mode = %q, want %q", mode, MigrationCheckModeWarn)
	}

	mode, err = NormalizeMigrationCheckMode("FAIL")
	if err != nil {
		t.Fatalf("NormalizeMigrationCheckMode returned error: %v", err)
	}
	if mode != MigrationCheckModeFail {
		t.Fatalf("mode = %q, want %q", mode, MigrationCheckModeFail)
	}

	if _, err := NormalizeMigrationCheckMode("strict"); err == nil {
		t.Fatal("NormalizeMigrationCheckMode accepted invalid mode")
	}
}

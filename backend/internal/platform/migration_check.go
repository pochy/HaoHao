package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MigrationCheckMode string

const (
	MigrationCheckModeWarn MigrationCheckMode = "warn"
	MigrationCheckModeFail MigrationCheckMode = "fail"
	MigrationCheckModeOff  MigrationCheckMode = "off"
)

type MigrationCheckResult struct {
	Mode            MigrationCheckMode
	CurrentVersion  int64
	ExpectedVersion int64
	Dirty           bool
	Status          string
	Err             error
}

func CheckMigrationVersion(ctx context.Context, pool *pgxpool.Pool, migrationsDir string, modeValue string, logger *slog.Logger) error {
	mode, err := NormalizeMigrationCheckMode(modeValue)
	if err != nil {
		return err
	}
	if mode == MigrationCheckModeOff {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}

	expectedVersion, err := MaxMigrationVersion(migrationsDir)
	if err != nil {
		result := MigrationCheckResult{Mode: mode, Status: "migration_files_unreadable", Err: err}
		logMigrationCheck(ctx, logger, result)
		if mode == MigrationCheckModeFail {
			return fmt.Errorf("check migration files: %w", err)
		}
		return nil
	}

	currentVersion, dirty, err := readDBMigrationVersion(ctx, pool)
	result := EvaluateMigrationCheck(mode, currentVersion, expectedVersion, dirty, err)
	if !result.NeedsLog() {
		return nil
	}

	logMigrationCheck(ctx, logger, result)
	if result.FailsStartup() {
		if result.Err != nil {
			return fmt.Errorf("check database migration version: %w", result.Err)
		}
		return fmt.Errorf("database migration check failed: status=%s current_version=%d expected_version=%d dirty=%t", result.Status, result.CurrentVersion, result.ExpectedVersion, result.Dirty)
	}
	return nil
}

func NormalizeMigrationCheckMode(value string) (MigrationCheckMode, error) {
	mode := MigrationCheckMode(strings.ToLower(strings.TrimSpace(value)))
	if mode == "" {
		mode = MigrationCheckModeWarn
	}
	switch mode {
	case MigrationCheckModeWarn, MigrationCheckModeFail, MigrationCheckModeOff:
		return mode, nil
	default:
		return "", fmt.Errorf("DB_MIGRATION_CHECK_MODE must be warn, fail, or off")
	}
}

func EvaluateMigrationCheck(mode MigrationCheckMode, currentVersion, expectedVersion int64, dirty bool, readErr error) MigrationCheckResult {
	result := MigrationCheckResult{
		Mode:            mode,
		CurrentVersion:  currentVersion,
		ExpectedVersion: expectedVersion,
		Dirty:           dirty,
		Err:             readErr,
	}
	switch {
	case mode == MigrationCheckModeOff:
		result.Status = "off"
	case readErr != nil:
		result.Status = "schema_migrations_unreadable"
	case dirty:
		result.Status = "dirty"
	case currentVersion < expectedVersion:
		result.Status = "behind"
	case currentVersion > expectedVersion:
		result.Status = "ahead"
	default:
		result.Status = "up_to_date"
	}
	return result
}

func (r MigrationCheckResult) NeedsLog() bool {
	return r.Status != "up_to_date" && r.Status != "off"
}

func (r MigrationCheckResult) FailsStartup() bool {
	return r.Mode == MigrationCheckModeFail && r.NeedsLog()
}

func MaxMigrationVersion(migrationsDir string) (int64, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return 0, err
	}

	pattern := regexp.MustCompile(`^(\d+)_.*\.up\.sql$`)
	var maxVersion int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := pattern.FindStringSubmatch(filepath.Base(entry.Name()))
		if len(matches) != 2 {
			continue
		}
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse migration version %q: %w", entry.Name(), err)
		}
		if version > maxVersion {
			maxVersion = version
		}
	}
	return maxVersion, nil
}

func readDBMigrationVersion(ctx context.Context, pool *pgxpool.Pool) (int64, bool, error) {
	var version int64
	var dirty bool
	err := pool.QueryRow(ctx, `select version, dirty from schema_migrations limit 1`).Scan(&version, &dirty)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return version, dirty, nil
}

func logMigrationCheck(ctx context.Context, logger *slog.Logger, result MigrationCheckResult) {
	attrs := []any{
		"log_type", "migration_check",
		"mode", string(result.Mode),
		"status", result.Status,
		"current_version", result.CurrentVersion,
		"expected_version", result.ExpectedVersion,
		"dirty", result.Dirty,
	}
	if result.Err != nil {
		attrs = append(attrs, "error", result.Err)
	}
	logger.ErrorContext(ctx, "database migration check failed", attrs...)
}

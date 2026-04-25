package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadReadsDotEnvFromWorkingDirectory(t *testing.T) {
	restoreEnv(t, "DATABASE_URL", "APP_NAME", "ZITADEL_SCOPES", "SCIM_RECONCILE_INTERVAL")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(`
DATABASE_URL=postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable
APP_NAME="From Dot Env"
ZITADEL_SCOPES="openid profile email"
SCIM_RECONCILE_INTERVAL=15m
`), 0o600); err != nil {
		t.Fatal(err)
	}

	chdir(t, dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DatabaseURL != "postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.AppName != "From Dot Env" {
		t.Fatalf("AppName = %q", cfg.AppName)
	}
	if cfg.ZitadelScopes != "openid profile email" {
		t.Fatalf("ZitadelScopes = %q", cfg.ZitadelScopes)
	}
	if cfg.SCIMReconcileInterval != 15*time.Minute {
		t.Fatalf("SCIMReconcileInterval = %s", cfg.SCIMReconcileInterval)
	}
}

func TestDotEnvDoesNotOverrideExistingEnvironment(t *testing.T) {
	restoreEnv(t, "DATABASE_URL")
	t.Setenv("DATABASE_URL", "postgres://from-env")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DATABASE_URL=postgres://from-dotenv\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	chdir(t, dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DatabaseURL != "postgres://from-env" {
		t.Fatalf("DatabaseURL = %q, want existing environment value", cfg.DatabaseURL)
	}
}

func TestLoadRejectsInvalidOperationalDuration(t *testing.T) {
	restoreEnv(t, "READINESS_TIMEOUT")
	t.Setenv("READINESS_TIMEOUT", "0s")

	dir := t.TempDir()
	chdir(t, dir)

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestParseDotEnvLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantKey   string
		wantValue string
		wantOK    bool
		wantErr   bool
	}{
		{
			name:      "quoted value",
			line:      `APP_NAME="HaoHao API"`,
			wantKey:   "APP_NAME",
			wantValue: "HaoHao API",
			wantOK:    true,
		},
		{
			name:      "unquoted value with spaces",
			line:      "ZITADEL_SCOPES=openid profile email",
			wantKey:   "ZITADEL_SCOPES",
			wantValue: "openid profile email",
			wantOK:    true,
		},
		{
			name:      "inline comment",
			line:      "AUTH_MODE=local # development",
			wantKey:   "AUTH_MODE",
			wantValue: "local",
			wantOK:    true,
		},
		{
			name:   "comment",
			line:   "# comment",
			wantOK: false,
		},
		{
			name:    "invalid",
			line:    "not dotenv",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, ok, err := parseDotEnvLine(tt.line)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseDotEnvLine() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDotEnvLine() error = %v", err)
			}
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if key != tt.wantKey || value != tt.wantValue {
				t.Fatalf("key, value = %q, %q; want %q, %q", key, value, tt.wantKey, tt.wantValue)
			}
		})
	}
}

func restoreEnv(t *testing.T, keys ...string) {
	t.Helper()

	previous := make(map[string]string, len(keys))
	existed := make(map[string]bool, len(keys))
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		previous[key] = value
		existed[key] = ok
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		for _, key := range keys {
			var err error
			if existed[key] {
				err = os.Setenv(key, previous[key])
			} else {
				err = os.Unsetenv(key)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
	})
}

func chdir(t *testing.T, dir string) {
	t.Helper()

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	})
}

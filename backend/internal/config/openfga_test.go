package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadOpenFGADefaultsDisabled(t *testing.T) {
	restoreEnv(t,
		"OPENFGA_ENABLED",
		"OPENFGA_API_URL",
		"OPENFGA_STORE_ID",
		"OPENFGA_AUTHORIZATION_MODEL_ID",
		"OPENFGA_API_TOKEN",
		"OPENFGA_TIMEOUT",
		"OPENFGA_FAIL_CLOSED",
	)
	chdir(t, t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.OpenFGA.Enabled {
		t.Fatal("OpenFGA.Enabled = true")
	}
	if cfg.OpenFGA.APIURL != "http://127.0.0.1:8088" {
		t.Fatalf("OpenFGA.APIURL = %q", cfg.OpenFGA.APIURL)
	}
	if cfg.OpenFGA.Timeout != 2*time.Second {
		t.Fatalf("OpenFGA.Timeout = %s", cfg.OpenFGA.Timeout)
	}
	if !cfg.OpenFGA.FailClosed {
		t.Fatal("OpenFGA.FailClosed = false")
	}
}

func TestLoadOpenFGAEnabledRequiresStoreAndModel(t *testing.T) {
	restoreEnv(t,
		"OPENFGA_ENABLED",
		"OPENFGA_API_URL",
		"OPENFGA_STORE_ID",
		"OPENFGA_AUTHORIZATION_MODEL_ID",
		"OPENFGA_TIMEOUT",
	)
	t.Setenv("OPENFGA_ENABLED", "true")
	t.Setenv("OPENFGA_API_URL", "http://127.0.0.1:8088")
	chdir(t, t.TempDir())

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil")
	}
	if !strings.Contains(err.Error(), "OPENFGA_STORE_ID") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadOpenFGAEnabled(t *testing.T) {
	restoreEnv(t,
		"OPENFGA_ENABLED",
		"OPENFGA_API_URL",
		"OPENFGA_STORE_ID",
		"OPENFGA_AUTHORIZATION_MODEL_ID",
		"OPENFGA_API_TOKEN",
		"OPENFGA_TIMEOUT",
		"OPENFGA_FAIL_CLOSED",
	)
	t.Setenv("OPENFGA_ENABLED", "true")
	t.Setenv("OPENFGA_API_URL", "http://127.0.0.1:8088/")
	t.Setenv("OPENFGA_STORE_ID", "store-1")
	t.Setenv("OPENFGA_AUTHORIZATION_MODEL_ID", "model-1")
	t.Setenv("OPENFGA_API_TOKEN", "token-1")
	t.Setenv("OPENFGA_TIMEOUT", "1500ms")
	t.Setenv("OPENFGA_FAIL_CLOSED", "false")
	chdir(t, t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.OpenFGA.Enabled {
		t.Fatal("OpenFGA.Enabled = false")
	}
	if cfg.OpenFGA.APIURL != "http://127.0.0.1:8088" {
		t.Fatalf("OpenFGA.APIURL = %q", cfg.OpenFGA.APIURL)
	}
	if cfg.OpenFGA.StoreID != "store-1" {
		t.Fatalf("OpenFGA.StoreID = %q", cfg.OpenFGA.StoreID)
	}
	if cfg.OpenFGA.AuthorizationModelID != "model-1" {
		t.Fatalf("OpenFGA.AuthorizationModelID = %q", cfg.OpenFGA.AuthorizationModelID)
	}
	if cfg.OpenFGA.APIToken != "token-1" {
		t.Fatalf("OpenFGA.APIToken = %q", cfg.OpenFGA.APIToken)
	}
	if cfg.OpenFGA.Timeout != 1500*time.Millisecond {
		t.Fatalf("OpenFGA.Timeout = %s", cfg.OpenFGA.Timeout)
	}
	if cfg.OpenFGA.FailClosed {
		t.Fatal("OpenFGA.FailClosed = true")
	}
}

func TestLoadOpenFGARejectsInvalidTimeout(t *testing.T) {
	restoreEnv(t, "OPENFGA_TIMEOUT")
	t.Setenv("OPENFGA_TIMEOUT", "0s")
	chdir(t, t.TempDir())

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
	}
}

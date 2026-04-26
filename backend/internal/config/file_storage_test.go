package config

import "testing"

func TestValidateFileStorageConfig(t *testing.T) {
	if err := validateFileStorageConfig("local", "", "", "", ""); err != nil {
		t.Fatalf("local config error = %v", err)
	}
	if err := validateFileStorageConfig("seaweedfs_s3", "http://127.0.0.1:8333", "haohao-drive-dev", "haohao", "haohao-secret"); err != nil {
		t.Fatalf("seaweedfs_s3 config error = %v", err)
	}
	if err := validateFileStorageConfig("seaweedfs_s3", "", "haohao-drive-dev", "haohao", "haohao-secret"); err == nil {
		t.Fatalf("missing endpoint did not fail")
	}
	if err := validateFileStorageConfig("unsupported", "", "", "", ""); err == nil {
		t.Fatalf("unsupported driver did not fail")
	}
}

package service

import "testing"

func TestDriveUserIsPlatformTenantAdmin(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  bool
	}{
		{name: "tenant admin", roles: []string{"tenant_admin"}, want: true},
		{name: "case and whitespace", roles: []string{" TODO_USER ", " Tenant_Admin "}, want: true},
		{name: "tenant scoped role name is not global role code", roles: []string{"tenant:acme:tenant_admin"}, want: false},
		{name: "other role", roles: []string{"todo_user"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := driveUserIsPlatformTenantAdmin(tt.roles); got != tt.want {
				t.Fatalf("driveUserIsPlatformTenantAdmin(%v) = %v, want %v", tt.roles, got, tt.want)
			}
		})
	}
}

func TestDriveShouldNormalizePendingScanStatus(t *testing.T) {
	tests := []struct {
		name   string
		policy DrivePolicy
		file   DriveFile
		want   bool
	}{
		{
			name:   "content scan disabled pending",
			policy: DrivePolicy{ContentScanEnabled: false},
			file:   DriveFile{ScanStatus: "pending"},
			want:   true,
		},
		{
			name:   "content scan enabled pending",
			policy: DrivePolicy{ContentScanEnabled: true},
			file:   DriveFile{ScanStatus: "pending"},
			want:   false,
		},
		{
			name:   "clean unchanged",
			policy: DrivePolicy{ContentScanEnabled: false},
			file:   DriveFile{ScanStatus: "clean"},
			want:   false,
		},
		{
			name:   "blocked unchanged",
			policy: DrivePolicy{ContentScanEnabled: false},
			file:   DriveFile{ScanStatus: "blocked"},
			want:   false,
		},
		{
			name:   "infected unchanged",
			policy: DrivePolicy{ContentScanEnabled: false},
			file:   DriveFile{ScanStatus: "infected"},
			want:   false,
		},
		{
			name:   "failed unchanged",
			policy: DrivePolicy{ContentScanEnabled: false},
			file:   DriveFile{ScanStatus: "failed"},
			want:   false,
		},
		{
			name:   "dlp blocked unchanged",
			policy: DrivePolicy{ContentScanEnabled: false},
			file:   DriveFile{ScanStatus: "pending", DLPBlocked: true},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := driveShouldNormalizePendingScanStatus(tt.policy, tt.file); got != tt.want {
				t.Fatalf("driveShouldNormalizePendingScanStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

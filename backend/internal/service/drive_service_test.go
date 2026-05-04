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

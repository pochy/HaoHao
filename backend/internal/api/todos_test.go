package api

import (
	"testing"

	"example.com/haohao/backend/internal/service"
)

func TestTenantHasRole(t *testing.T) {
	tenant := service.TenantAccess{Roles: []string{"docs_reader", "Todo_User"}}
	if !tenantHasRole(tenant, "todo_user") {
		t.Fatal("tenantHasRole() = false, want true")
	}
	if tenantHasRole(tenant, "machine_client_admin") {
		t.Fatal("tenantHasRole(global-only role) = true, want false")
	}
}

func TestActiveTenantHasAnyRole(t *testing.T) {
	tenant := service.TenantAccess{Roles: []string{"tenant_admin"}}
	if !activeTenantHasAnyRole(tenant, []string{"data_pipeline_user", "tenant_admin"}) {
		t.Fatal("activeTenantHasAnyRole() = false, want true")
	}
	if activeTenantHasAnyRole(tenant, []string{"data_pipeline_user", "docs_reader"}) {
		t.Fatal("activeTenantHasAnyRole() = true, want false")
	}
}

func TestAuthContextHasTenantRole(t *testing.T) {
	authCtx := service.AuthContext{Tenants: []service.TenantAccess{
		{Slug: "acme", Roles: []string{"tenant_admin"}},
		{Slug: "beta", Roles: []string{"todo_user"}},
	}}
	if !authContextHasTenantRole(authCtx, "acme", "tenant_admin") {
		t.Fatal("authContextHasTenantRole(acme tenant_admin) = false, want true")
	}
	if authContextHasTenantRole(authCtx, "beta", "tenant_admin") {
		t.Fatal("authContextHasTenantRole(beta tenant_admin) = true, want false")
	}
}

func TestCanAccessSystemJobs(t *testing.T) {
	tests := []struct {
		name    string
		authCtx service.AuthContext
		want    bool
	}{
		{
			name:    "platform admin",
			authCtx: service.AuthContext{Roles: []string{"tenant_admin"}, ActiveTenant: &service.TenantAccess{Slug: "beta", Roles: []string{"todo_user"}}},
			want:    true,
		},
		{
			name:    "active tenant admin",
			authCtx: service.AuthContext{ActiveTenant: &service.TenantAccess{Slug: "acme", Roles: []string{"tenant_admin"}}},
			want:    true,
		},
		{
			name:    "tenant admin on another tenant is not enough",
			authCtx: service.AuthContext{ActiveTenant: &service.TenantAccess{Slug: "beta", Roles: []string{"todo_user"}}, Tenants: []service.TenantAccess{{Slug: "acme", Roles: []string{"tenant_admin"}}}},
			want:    false,
		},
		{
			name:    "no active tenant",
			authCtx: service.AuthContext{Roles: []string{"docs_reader"}},
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canAccessSystemJobs(tt.authCtx); got != tt.want {
				t.Fatalf("canAccessSystemJobs() = %v, want %v", got, tt.want)
			}
		})
	}
}

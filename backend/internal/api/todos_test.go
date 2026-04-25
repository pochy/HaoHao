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

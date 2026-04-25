package service

import (
	"reflect"
	"testing"

	db "example.com/haohao/backend/internal/db"
)

func TestParseTenantRoleClaims(t *testing.T) {
	got := ParseTenantRoleClaims([]string{
		"tenant:acme:todo_user",
		"tenant:acme:todo_user",
		"tenant:beta:docs_reader",
		"external_api_user",
		"tenant:acme:external_api_user",
		"tenant::todo_user",
		"tenant:bad",
	})

	want := []TenantRoleClaim{
		{TenantSlug: "acme", RoleCode: "todo_user"},
		{TenantSlug: "beta", RoleCode: "docs_reader"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseTenantRoleClaims() = %#v, want %#v", got, want)
	}
}

func TestTenantAccessFromRowsAppliesOverrides(t *testing.T) {
	rows := []db.ListTenantMembershipRowsByUserIDRow{
		{TenantID: 1, TenantSlug: "acme", TenantDisplayName: "Acme", TenantActive: true, RoleCode: "todo_user", MembershipActive: true},
		{TenantID: 1, TenantSlug: "acme", TenantDisplayName: "Acme", TenantActive: true, RoleCode: "docs_reader", MembershipActive: true},
	}

	got := tenantAccessFromRows(rows, []db.ListTenantRoleOverridesByUserIDRow{
		{TenantID: 1, TenantSlug: "acme", RoleCode: "todo_user", Effect: "deny"},
		{TenantID: 1, TenantSlug: "acme", RoleCode: "docs_reader", Effect: "allow"},
	})

	want := []TenantAccess{{
		ID:          1,
		Slug:        "acme",
		DisplayName: "Acme",
		Roles:       []string{"docs_reader"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tenant access = %#v, want %#v", got, want)
	}
}

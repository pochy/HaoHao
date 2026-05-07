package service

import "testing"

func TestDataScopeManagerTuplesIncludesScopeAndMemberTuples(t *testing.T) {
	tuples := dataScopeManagerTuples("scope-public-id", "group-public-id", []DatasetPermissionGroupMember{
		{PublicID: "user-public-id"},
		{PublicID: "   "},
	})

	want := []OpenFGATuple{
		{User: "dataset_group:group-public-id#member", Relation: "owner", Object: "data_scope:scope-public-id"},
		{User: "dataset_group:group-public-id#member", Relation: "dataset_creator", Object: "data_scope:scope-public-id"},
		{User: "dataset_group:group-public-id#member", Relation: "work_table_creator", Object: "data_scope:scope-public-id"},
		{User: "dataset_group:group-public-id#member", Relation: "pipeline_creator", Object: "data_scope:scope-public-id"},
		{User: "user:user-public-id", Relation: "member", Object: "dataset_group:group-public-id"},
	}
	if len(tuples) != len(want) {
		t.Fatalf("tuple count = %d, want %d: %#v", len(tuples), len(want), tuples)
	}
	for i := range want {
		if tuples[i] != want[i] {
			t.Fatalf("tuple[%d] = %#v, want %#v", i, tuples[i], want[i])
		}
	}
}

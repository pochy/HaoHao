package service

import (
	"errors"
	"testing"
	"time"
)

func TestNormalizeDataPipelineListInputDefaults(t *testing.T) {
	input, cursor, err := normalizeDataPipelineListInput(DataPipelineListInput{})
	if err != nil {
		t.Fatalf("normalizeDataPipelineListInput() error = %v", err)
	}
	if input.Limit != 25 {
		t.Fatalf("Limit = %d, want 25", input.Limit)
	}
	if input.Publication != "all" {
		t.Fatalf("Publication = %q, want all", input.Publication)
	}
	if input.ScheduleState != "all" {
		t.Fatalf("ScheduleState = %q, want all", input.ScheduleState)
	}
	if input.Sort != "updated_desc" {
		t.Fatalf("Sort = %q, want updated_desc", input.Sort)
	}
	if cursor.ID != 0 {
		t.Fatalf("cursor ID = %d, want 0", cursor.ID)
	}
}

func TestNormalizeDataPipelineListInputRejectsInvalidValues(t *testing.T) {
	cases := []DataPipelineListInput{
		{Status: "archived"},
		{Publication: "draft"},
		{RunStatus: "ready"},
		{ScheduleState: "paused"},
		{Sort: "updated"},
	}
	for _, tc := range cases {
		_, _, err := normalizeDataPipelineListInput(tc)
		if !errors.Is(err, ErrInvalidDataPipelineInput) {
			t.Fatalf("normalizeDataPipelineListInput(%+v) error = %v, want ErrInvalidDataPipelineInput", tc, err)
		}
	}
}

func TestDataPipelineListCursorSortMismatch(t *testing.T) {
	encoded, err := encodeDataPipelineListCursor("updated_desc", DataPipeline{
		ID:        42,
		Name:      "Pipeline",
		UpdatedAt: time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("encodeDataPipelineListCursor() error = %v", err)
	}

	_, _, err = normalizeDataPipelineListInput(DataPipelineListInput{Sort: "created_desc", Cursor: encoded})
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("normalizeDataPipelineListInput() error = %v, want ErrInvalidCursor", err)
	}
}

func TestDataPipelineListNameCursorRoundTrip(t *testing.T) {
	encoded, err := encodeDataPipelineListCursor("name_asc", DataPipeline{ID: 7, Name: "Zeta Pipeline"})
	if err != nil {
		t.Fatalf("encodeDataPipelineListCursor() error = %v", err)
	}
	_, cursor, err := normalizeDataPipelineListInput(DataPipelineListInput{Sort: "name_asc", Cursor: encoded})
	if err != nil {
		t.Fatalf("normalizeDataPipelineListInput() error = %v", err)
	}
	if cursor.ID != 7 || cursor.Text != "zeta pipeline" {
		t.Fatalf("cursor = %+v, want ID 7 and lower-case text", cursor)
	}
}

package domain

import (
	"encoding/json"
	"testing"
)

func TestTaskStatusUsesDoingInJSON(t *testing.T) {
	data, err := json.Marshal(map[TaskStatus]Task{
		TaskStatusDoing: {Id: "1", Content: "active", Status: TaskStatusDoing},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"doing":{"id":"1","content":"active","status":"doing","is_starred":false,"created_at":0}}`
	if string(data) != want {
		t.Fatalf("JSON = %s, want %s", data, want)
	}
}

func TestTaskStatusReadsLegacyNumericJSON(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want TaskStatus
	}{
		{`1`, TaskStatusDoing},
		{`3`, TaskStatusArchived},
	} {
		var status TaskStatus
		if err := json.Unmarshal([]byte(tc.raw), &status); err != nil {
			t.Fatal(err)
		}
		if status != tc.want {
			t.Fatalf("status for %s = %v, want %v", tc.raw, status, tc.want)
		}
	}
}

func TestParseTaskStatus(t *testing.T) {
	for _, tc := range []struct {
		input   string
		want    TaskStatus
		wantErr bool
	}{
		{"todo", TaskStatusTodo, false},
		{"doing", TaskStatusDoing, false},
		{"done", TaskStatusDone, false},
		{"archived", TaskStatusArchived, false},
		{"  ARCHIVED  ", TaskStatusArchived, false},
		{"deleted", TaskStatusTodo, true},
	} {
		got, err := ParseTaskStatus(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("ParseTaskStatus(%q) succeeded, want an error", tc.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseTaskStatus(%q): %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("ParseTaskStatus(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestArchivedStatusRoundTripsThroughJSON(t *testing.T) {
	data, err := json.Marshal(Task{Id: "9", Content: "old", Status: TaskStatusArchived, ArchivedAt: 42})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"9","content":"old","status":"archived","is_starred":false,"created_at":0,"archived_at":42}`
	if string(data) != want {
		t.Fatalf("JSON = %s, want %s", data, want)
	}
	var decoded Task
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Status != TaskStatusArchived || decoded.ArchivedAt != 42 {
		t.Fatalf("decoded = %+v", decoded)
	}
}

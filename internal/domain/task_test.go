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
	var status TaskStatus
	if err := json.Unmarshal([]byte(`1`), &status); err != nil {
		t.Fatal(err)
	}
	if status != TaskStatusDoing {
		t.Fatalf("status = %v, want doing", status)
	}
}

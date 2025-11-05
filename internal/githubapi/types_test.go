package githubapi

import (
	"testing"
	"time"
)

func TestMapWorkflowRunDurationFallback(t *testing.T) {
	start := time.Date(2025, 5, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(45 * time.Minute)
	json := workflowRunJSON{
		ID:           1,
		WorkflowID:   10,
		Name:         "build",
		Status:       "completed",
		Conclusion:   "success",
		CreatedAt:    start.Add(-time.Minute),
		RunStartedAt: &start,
		UpdatedAt:    end,
		RunNumber:    5,
	}

	run := mapWorkflowRun(json)
	if run.Duration != 45*time.Minute {
		t.Fatalf("expected duration 45m from timestamps, got %s", run.Duration)
	}
	if run.RunStartedAt != start {
		t.Fatalf("expected runStartedAt preserved")
	}
}

func TestMapWorkflowJob(t *testing.T) {
	start := time.Date(2025, 5, 2, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	json := workflowJobJSON{
		ID:          99,
		Name:        "job",
		Status:      "completed",
		Conclusion:  "failure",
		StartedAt:   &start,
		CompletedAt: &end,
		Labels:      []string{"ubuntu-latest"},
	}

	job := mapWorkflowJob(json)
	if job.Duration() != 30*time.Minute {
		t.Fatalf("expected duration 30m, got %s", job.Duration())
	}
	if job.Name != "job" || job.Conclusion != "failure" {
		t.Fatalf("unexpected job mapping %#v", job)
	}
}

package metrics

import (
	"testing"
	"time"

	"github.com/JohnTitor/gh-actrics/internal/githubapi"
)

func TestAggregateBasic(t *testing.T) {
	base := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)

	workflowA := githubapi.Workflow{ID: 1, Name: "build"}
	workflowB := githubapi.Workflow{ID: 2, Name: "test"}

	runA1 := githubapi.WorkflowRun{
		ID:           101,
		WorkflowID:   workflowA.ID,
		Name:         workflowA.Name,
		Status:       "completed",
		Conclusion:   "success",
		CreatedAt:    base.Add(2 * time.Hour),
		RunStartedAt: base.Add(2 * time.Hour),
		UpdatedAt:    base.Add(150 * time.Minute),
		Duration:     30 * time.Minute,
	}
	runA2 := githubapi.WorkflowRun{
		ID:           102,
		WorkflowID:   workflowA.ID,
		Name:         workflowA.Name,
		Status:       "completed",
		Conclusion:   "failure",
		CreatedAt:    base.Add(6 * time.Hour),
		RunStartedAt: base.Add(6 * time.Hour),
		UpdatedAt:    base.Add(7 * time.Hour),
		Duration:     1 * time.Hour,
	}
	runB1 := githubapi.WorkflowRun{
		ID:           201,
		WorkflowID:   workflowB.ID,
		Name:         workflowB.Name,
		Status:       "completed",
		Conclusion:   "success",
		CreatedAt:    base.Add(26 * time.Hour),
		RunStartedAt: base.Add(26 * time.Hour),
		UpdatedAt:    base.Add(28 * time.Hour),
		Duration:     2 * time.Hour,
	}

	jobA := githubapi.WorkflowJob{
		ID:          1001,
		Name:        "job-a",
		Labels:      []string{"ubuntu-latest", "self-hosted", "self-hosted"},
		StartedAt:   base.Add(2 * time.Hour),
		CompletedAt: base.Add(2*time.Hour + 20*time.Minute),
	}
	jobB := githubapi.WorkflowJob{
		ID:          1002,
		Name:        "job-b",
		Labels:      []string{"ubuntu-latest"},
		StartedAt:   base.Add(6 * time.Hour),
		CompletedAt: base.Add(6*time.Hour + 30*time.Minute),
	}
	jobC := githubapi.WorkflowJob{
		ID:          2001,
		Name:        "job-c",
		Labels:      []string{"macos"},
		StartedAt:   base.Add(26 * time.Hour),
		CompletedAt: base.Add(26*time.Hour + time.Hour),
	}

	records := []RunRecord{
		{Workflow: workflowA, Run: runA1, Jobs: []githubapi.WorkflowJob{jobA}},
		{Workflow: workflowA, Run: runA2, Jobs: []githubapi.WorkflowJob{jobB}},
		{Workflow: workflowB, Run: runB1, Jobs: []githubapi.WorkflowJob{jobC}},
	}

	from := base
	to := base.Add(48 * time.Hour)

	rows := Aggregate(records, from, to)

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	first := rows[0]
	if first.Workflow != "build" {
		t.Fatalf("expected workflow build, got %s", first.Workflow)
	}
	if first.Runs != 2 {
		t.Fatalf("expected 2 runs, got %d", first.Runs)
	}
	if first.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", first.Failed)
	}
	if first.TotalDuration != 90*time.Minute {
		t.Fatalf("expected total duration 90m, got %s", first.TotalDuration)
	}
	expectedAvg := 45 * time.Minute
	if first.AvgDuration != expectedAvg {
		t.Fatalf("expected avg duration %s, got %s", expectedAvg, first.AvgDuration)
	}
	if want := 0.5; first.FailureRate != want {
		t.Fatalf("expected failure rate %v, got %v", want, first.FailureRate)
	}

	if len(first.RunnerSummary) != 2 {
		t.Fatalf("expected 2 runner entries, got %d", len(first.RunnerSummary))
	}
	if first.RunnerSummary[0].Label != "ubuntu-latest" {
		t.Fatalf("expected runner ubuntu-latest first, got %s", first.RunnerSummary[0].Label)
	}
	if first.RunnerSummary[0].Runs != 2 {
		t.Fatalf("expected ubuntu-latest runs 2, got %d", first.RunnerSummary[0].Runs)
	}
	if first.RunnerSummary[0].Duration != 50*time.Minute {
		t.Fatalf("expected ubuntu-latest duration 50m, got %s", first.RunnerSummary[0].Duration)
	}
	if first.RunnerSummary[1].Label != "self-hosted" {
		t.Fatalf("expected runner self-hosted second, got %s", first.RunnerSummary[1].Label)
	}
	if first.RunnerSummary[1].Runs != 1 {
		t.Fatalf("expected self-hosted runs 1, got %d", first.RunnerSummary[1].Runs)
	}

	second := rows[1]
	if second.Workflow != "test" {
		t.Fatalf("expected workflow test, got %s", second.Workflow)
	}
	if second.TotalDuration != 2*time.Hour {
		t.Fatalf("expected total duration 2h, got %s", second.TotalDuration)
	}
	if second.Runs != 1 {
		t.Fatalf("expected runs=1, got %d", second.Runs)
	}
	if second.FailureRate != 0 {
		t.Fatalf("expected failure rate 0, got %v", second.FailureRate)
	}
}

func TestAggregateFiltersOutsideRange(t *testing.T) {
	base := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	workflow := githubapi.Workflow{ID: 1, Name: "build"}
	run := githubapi.WorkflowRun{
		ID:         1,
		WorkflowID: 1,
		Status:     "completed",
		Conclusion: "success",
		CreatedAt:  base.Add(-2 * time.Hour),
		Duration:   5 * time.Minute,
	}

	rows := Aggregate([]RunRecord{{Workflow: workflow, Run: run}}, base, base.Add(24*time.Hour))
	if len(rows) != 0 {
		t.Fatalf("expected no rows, got %d", len(rows))
	}
}

func TestAggregateMultipleRuns(t *testing.T) {
	base := time.Date(2025, 3, 3, 12, 0, 0, 0, time.UTC)
	workflow := githubapi.Workflow{ID: 1, Name: "deploy"}

	records := []RunRecord{
		{Workflow: workflow, Run: githubapi.WorkflowRun{ID: 1, WorkflowID: 1, Status: "completed", Conclusion: "success", CreatedAt: base, Duration: time.Hour}},
		{Workflow: workflow, Run: githubapi.WorkflowRun{ID: 2, WorkflowID: 1, Status: "completed", Conclusion: "success", CreatedAt: base.AddDate(0, 0, 6), Duration: 2 * time.Hour}},
		{Workflow: workflow, Run: githubapi.WorkflowRun{ID: 3, WorkflowID: 1, Status: "completed", Conclusion: "success", CreatedAt: base.AddDate(0, 1, 0), Duration: 3 * time.Hour}},
	}

	from := base.Add(-time.Hour)
	to := base.AddDate(0, 1, 1)

	rows := Aggregate(records, from, to)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row (grouped by workflow), got %d", len(rows))
	}
	if rows[0].Runs != 3 {
		t.Fatalf("expected runs=3, got %d", rows[0].Runs)
	}
	if rows[0].TotalDuration != 6*time.Hour {
		t.Fatalf("expected total 6h, got %s", rows[0].TotalDuration)
	}
}

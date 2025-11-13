package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/JohnTitor/gh-actrics/internal/githubapi"
	"github.com/JohnTitor/gh-actrics/internal/metrics"
)

func TestRenderMarkdownSummary(t *testing.T) {
	rows := []metrics.SummaryRow{{
		Workflow:      "build",
		WorkflowID:    1,
		Runs:          2,
		Failed:        1,
		FailureRate:   0.5,
		AvgDuration:   time.Minute,
		TotalDuration: 2 * time.Minute,
		RunnerSummary: []metrics.RunnerUsage{
			{Label: "ubuntu-latest", Runs: 2, Duration: 2 * time.Minute},
		},
		Jobs: []metrics.JobSummaryRow{{
			Job:           "test",
			Runs:          2,
			Failed:        0,
			FailureRate:   0,
			AvgDuration:   30 * time.Second,
			TotalDuration: time.Minute,
		}},
	}}

	var buf bytes.Buffer
	renderMarkdownSummary(&buf, rows)
	got := strings.TrimSpace(buf.String())

	const want = `# Workflow Execution Summary

| Workflow | Runs | Failed | Failure Rate | Avg Duration | Total Duration | Top Runners |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| build | 2 | 1 | 50.0% | 1m0s | 2m0s | ubuntu-latest(2/2m0s) |

## Jobs for build

| Job | Runs | Failed | Failure Rate | Avg Duration | Total Duration | Top Runners |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| test | 2 | 0 | 0% | 30s | 1m0s | - |`

	if got != want {
		t.Fatalf("markdown summary mismatch:\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestRenderRunsMarkdown(t *testing.T) {
	rows := []runRow{{
		WorkflowID:   42,
		WorkflowName: "build",
		RunID:        12345,
		Status:       "completed",
		Conclusion:   "success",
		CreatedAt:    time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
		Duration:     90 * time.Second,
		RunAttempt:   2,
		RunNumber:    7,
		HeadBranch:   "main",
	}}

	var buf bytes.Buffer
	renderRunsMarkdown(&buf, rows)
	got := strings.TrimSpace(buf.String())

	const want = `# Workflow Runs

| Workflow | Run ID | Status | Conclusion | Duration | Branch | Run # | Attempt | Created |
| --- | --- | --- | --- | --- | --- | ---: | ---: | --- |
| build | 12345 | completed | success | 1m30s | main | 7 | 2 | 2025-01-02T03:04:05Z |`

	if got != want {
		t.Fatalf("markdown runs mismatch:\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestRenderMarkdownWorkflows(t *testing.T) {
	workflows := []githubapi.Workflow{{
		ID:    101,
		Name:  "CI",
		Path:  ".github/workflows/ci.yml",
		State: "active",
	}}

	var buf bytes.Buffer
	renderMarkdownWorkflows(&buf, workflows, "owner", "repo")
	got := strings.TrimSpace(buf.String())

	const want = `# Workflows in owner/repo

| ID | Name | Path | State |
| ---: | --- | --- | --- |
| 101 | CI | .github/workflows/ci.yml | active |`

	if got != want {
		t.Fatalf("markdown workflows mismatch:\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JohnTitor/gh-actrics/internal/githubapi"
	"github.com/JohnTitor/gh-actrics/internal/metrics"
)

func TestWorkflowMatches(t *testing.T) {
	workflow := githubapi.Workflow{ID: 42, Name: "Build", Path: ".github/workflows/build.yaml"}
	selectors := []string{"build", "deploy"}
	if !workflowMatches(workflow, selectors) {
		t.Fatalf("expected match by name")
	}

	selectors = []string{"build.yaml"}
	if !workflowMatches(workflow, selectors) {
		t.Fatalf("expected match by base filename")
	}

	selectors = []string{"42"}
	if !workflowMatches(workflow, selectors) {
		t.Fatalf("expected match by ID")
	}

	selectors = []string{"test"}
	if workflowMatches(workflow, selectors) {
		t.Fatalf("did not expect match for non-existent selector")
	}
}

func TestFilterWorkflows(t *testing.T) {
	workflows := []githubapi.Workflow{{ID: 1, Name: "build"}, {ID: 2, Name: "test"}}

	matched, err := filterWorkflows(workflows, []string{"test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matched) != 1 || matched[0].Name != "test" {
		t.Fatalf("unexpected match result: %#v", matched)
	}

	if _, err := filterWorkflows(nil, []string{}); err == nil {
		t.Fatalf("expected error when repository has no workflows")
	}
}

func TestWriteCSV(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "summary.csv")

	rows := []metrics.SummaryRow{{
		Workflow:      "build",
		WorkflowID:    1,
		Runs:          2,
		Failed:        1,
		FailureRate:   0.5,
		AvgDuration:   30 * time.Minute,
		TotalDuration: 60 * time.Minute,
	}}

	if err := writeCSV(rows, path); err != nil {
		t.Fatalf("writeCSV failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read csv: %v", err)
	}
	if len(content) == 0 {
		t.Fatalf("expected csv content")
	}
	if !strings.Contains(string(content), "workflow_id") {
		t.Fatalf("expected header in csv, got %s", content)
	}
}

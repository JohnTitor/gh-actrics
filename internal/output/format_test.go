package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/JohnTitor/gh-actrics/internal/metrics"
	"github.com/cli/go-gh/v2/pkg/tableprinter"
)

func TestFormatHelpers(t *testing.T) {
	if got := formatDuration(0); got != "-" {
		t.Fatalf("expected '-' for zero duration, got %s", got)
	}
	if got := formatDuration(1500 * time.Millisecond); got != "1.5s" {
		t.Fatalf("unexpected duration format: %s", got)
	}

	if got := formatFailureRate(0); got != "0%" {
		t.Fatalf("expected 0%%, got %s", got)
	}
	if got := formatFailureRate(0.237); got != "23.7%" {
		t.Fatalf("unexpected failure rate: %s", got)
	}

	summary := formatRunnerSummary([]metrics.RunnerUsage{{Label: "ubuntu", Runs: 2, Duration: 5 * time.Minute}, {Label: "macos", Runs: 1, Duration: 2 * time.Minute}}, 1)
	if !strings.Contains(summary, "...") {
		t.Fatalf("expected ellipsis for trimmed runner summary: %s", summary)
	}
}

func TestWriteSummaryTable(t *testing.T) {
	buf := &bytes.Buffer{}
	tp := tableprinter.New(buf, false, 120)
	rows := []metrics.SummaryRow{{
		Workflow:      "build",
		WorkflowID:    123,
		Runs:          3,
		Failed:        1,
		FailureRate:   1.0 / 3,
		AvgDuration:   10 * time.Minute,
		TotalDuration: 30 * time.Minute,
	}}
	WriteSummaryTable(tp, rows)
	if err := tp.Render(); err != nil {
		t.Fatalf("render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "build") {
		t.Fatalf("expected table output to contain workflow name, got %s", out)
	}
}

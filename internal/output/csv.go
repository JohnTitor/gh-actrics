package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"github.com/JohnTitor/gh-actrics/internal/metrics"
)

// WriteSummaryCSV writes summary rows into CSV format.
func WriteSummaryCSV(w io.Writer, rows []metrics.SummaryRow) error {
	writer := csv.NewWriter(w)
	header := []string{
		"workflow",
		"workflow_id",
		"runs",
		"failed",
		"failure_rate",
		"avg_duration_ms",
		"total_duration_ms",
		"runner_summary",
		"jobs_json",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, row := range rows {
		jobsJSON := "[]"
		if len(row.Jobs) > 0 {
			raw, err := json.Marshal(row.Jobs)
			if err != nil {
				return fmt.Errorf("failed to marshal job summary for workflow %s: %w", row.Workflow, err)
			}
			jobsJSON = string(raw)
		}

		record := []string{
			row.Workflow,
			fmt.Sprintf("%d", row.WorkflowID),
			fmt.Sprintf("%d", row.Runs),
			fmt.Sprintf("%d", row.Failed),
			fmt.Sprintf("%.4f", row.FailureRate),
			fmt.Sprintf("%d", row.AvgDuration.Milliseconds()),
			fmt.Sprintf("%d", row.TotalDuration.Milliseconds()),
			formatRunnerSummary(row.RunnerSummary, len(row.RunnerSummary)),
			jobsJSON,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}

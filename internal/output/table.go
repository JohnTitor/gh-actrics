package output

import (
	"fmt"

	"github.com/JohnTitor/gh-actrics/internal/metrics"
	"github.com/cli/go-gh/v2/pkg/tableprinter"
)

// WriteSummaryTable renders summary rows into a table printer.
func WriteSummaryTable(tp tableprinter.TablePrinter, rows []metrics.SummaryRow) {
	tp.AddHeader([]string{"Workflow", "Runs", "Failed", "Failure%", "Avg", "Total", "Top runners"})

	for _, row := range rows {
		addRow(tp,
			row.Workflow,
			fmt.Sprintf("%d", row.Runs),
			fmt.Sprintf("%d", row.Failed),
			formatFailureRate(row.FailureRate),
			formatDuration(row.AvgDuration),
			formatDuration(row.TotalDuration),
			formatRunnerSummary(row.RunnerSummary, 3),
		)
	}

	if len(rows) == 0 {
		addRow(tp, "(no data)")
	}
}

func addRow(tp tableprinter.TablePrinter, fields ...string) {
	for _, field := range fields {
		tp.AddField(field)
	}
	tp.EndRow()
}

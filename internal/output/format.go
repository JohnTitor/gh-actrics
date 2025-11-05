package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/JohnTitor/gh-actrics/internal/metrics"
)

// FormatDuration formats a duration for display
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	return d.Truncate(time.Millisecond).String()
}

func formatDuration(d time.Duration) string {
	return FormatDuration(d)
}

// FormatFailureRate formats a failure rate as a percentage
func FormatFailureRate(rate float64) string {
	if rate <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%.1f%%", rate*100)
}

func formatFailureRate(rate float64) string {
	return FormatFailureRate(rate)
}

// FormatRunnerSummary formats runner usage summary
func FormatRunnerSummary(usages []metrics.RunnerUsage, limit int) string {
	if len(usages) == 0 {
		return "-"
	}
	end := limit
	if end > len(usages) {
		end = len(usages)
	}
	parts := make([]string, 0, end)
	for i := 0; i < end; i++ {
		usage := usages[i]
		parts = append(parts, fmt.Sprintf("%s(%d/%s)", usage.Label, usage.Runs, FormatDuration(usage.Duration)))
	}
	if len(usages) > limit {
		parts = append(parts, "...")
	}
	return strings.Join(parts, ", ")
}

func formatRunnerSummary(usages []metrics.RunnerUsage, limit int) string {
	return FormatRunnerSummary(usages, limit)
}

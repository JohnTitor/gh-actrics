package metrics

import (
	"sort"
	"strings"
	"time"

	"github.com/JohnTitor/gh-actrics/internal/githubapi"
)

// RunRecord bundles a workflow run with metadata needed for aggregation.
type RunRecord struct {
	Workflow githubapi.Workflow
	Run      githubapi.WorkflowRun
	Jobs     []githubapi.WorkflowJob
}

// RunnerUsage summarizes usage per runner label.
type RunnerUsage struct {
	Label    string        `json:"label"`
	Runs     int           `json:"runs"`
	Duration time.Duration `json:"duration"`
}

// SummaryRow represents aggregated metrics for a workflow.
type SummaryRow struct {
	Workflow      string        `json:"workflow"`
	WorkflowID    int64         `json:"workflow_id"`
	Runs          int           `json:"runs"`
	Failed        int           `json:"failed"`
	FailureRate   float64       `json:"failure_rate"`
	AvgDuration   time.Duration `json:"avg_duration"`
	TotalDuration time.Duration `json:"total_duration"`
	RunnerSummary []RunnerUsage `json:"runner_summary"`
}

// Aggregate computes summary rows for the provided records, grouped by workflow.
func Aggregate(records []RunRecord, from, to time.Time) []SummaryRow {
	workflowStats := make(map[int64]*workflowStat)

	for _, rec := range records {
		runTime := rec.Run.RunStartedAt
		if runTime.IsZero() {
			runTime = rec.Run.CreatedAt
		}
		if runTime.Before(from) || runTime.After(to) {
			continue
		}

		stat, ok := workflowStats[rec.Workflow.ID]
		if !ok {
			stat = &workflowStat{
				workflow:   rec.Workflow.Name,
				workflowID: rec.Workflow.ID,
				runner:     make(map[string]*runnerStat),
			}
			workflowStats[rec.Workflow.ID] = stat
		}

		stat.runs++

		if isFailure(rec.Run.Conclusion, rec.Run.Status) {
			stat.failed++
		}

		duration := rec.Run.Duration
		stat.duration += duration

		if len(rec.Jobs) > 0 {
			accumulateRunnerStats(stat.runner, rec.Jobs)
		}
	}

	rows := make([]SummaryRow, 0, len(workflowStats))
	for _, stat := range workflowStats {
		row := SummaryRow{
			Workflow:      stat.workflow,
			WorkflowID:    stat.workflowID,
			Runs:          stat.runs,
			Failed:        stat.failed,
			TotalDuration: stat.duration,
		}

		if stat.runs > 0 {
			row.AvgDuration = time.Duration(int64(stat.duration) / int64(stat.runs))
			row.FailureRate = float64(stat.failed) / float64(stat.runs)
		}

		row.RunnerSummary = flattenRunnerStats(stat.runner)
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Workflow < rows[j].Workflow
	})

	return rows
}

type workflowStat struct {
	workflow   string
	workflowID int64
	runs       int
	failed     int
	duration   time.Duration
	runner     map[string]*runnerStat
}

type runnerStat struct {
	duration time.Duration
	runs     int
}

func accumulateRunnerStats(stats map[string]*runnerStat, jobs []githubapi.WorkflowJob) {
	for _, job := range jobs {
		duration := job.Duration()
		if duration <= 0 {
			continue
		}

		labels := job.Labels
		if len(labels) == 0 {
			labels = []string{"(unknown)"}
		}
		seen := make(map[string]struct{})
		for _, label := range labels {
			label = strings.TrimSpace(label)
			if label == "" {
				label = "(unknown)"
			}
			if _, ok := seen[label]; ok {
				continue
			}
			seen[label] = struct{}{}
			stat, ok := stats[label]
			if !ok {
				stat = &runnerStat{}
				stats[label] = stat
			}
			stat.duration += duration
			stat.runs++
		}
	}
}

func flattenRunnerStats(stats map[string]*runnerStat) []RunnerUsage {
	if len(stats) == 0 {
		return nil
	}

	out := make([]RunnerUsage, 0, len(stats))
	for label, stat := range stats {
		out = append(out, RunnerUsage{
			Label:    label,
			Runs:     stat.runs,
			Duration: stat.duration,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Duration == out[j].Duration {
			return out[i].Label < out[j].Label
		}
		return out[i].Duration > out[j].Duration
	})

	return out
}

func isFailure(conclusion, status string) bool {
	v := strings.ToLower(conclusion)
	switch v {
	case "failure", "failed", "cancelled", "timed_out", "action_required", "stale":
		return true
	case "":
		// fall back to status when conclusion is empty (in-progress runs)
		s := strings.ToLower(status)
		return s == "failure"
	default:
		return false
	}
}

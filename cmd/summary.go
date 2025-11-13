package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JohnTitor/gh-actrics/internal/githubapi"
	"github.com/JohnTitor/gh-actrics/internal/metrics"
	"github.com/JohnTitor/gh-actrics/internal/output"
	"github.com/JohnTitor/gh-actrics/internal/util"
	"github.com/briandowns/spinner"
	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	flagSummaryRuns = "runs"
)

func newSummaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary <owner>/<repo>",
		Short: "Aggregate workflow metrics by workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			owner, repo, err := util.ParseRepo(args[0])
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			from, to, err := resolveTimeRange(now, viper.GetString(flagFrom), viper.GetString(flagTo), viper.GetString(flagLast))
			if err != nil {
				return err
			}

			cacheTTL := viper.GetDuration(flagCacheTTL)
			enableCache := cacheTTL > 0 && !viper.GetBool(flagNoCache)

			client, err := githubapi.NewClient(githubapi.Options{CacheTTL: cacheTTL, EnableCache: enableCache})
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			allWorkflows, err := client.ListWorkflows(ctx, owner, repo)
			if err != nil {
				return fmt.Errorf("failed to list workflows for %s/%s: %w", owner, repo, err)
			}

			// Filter out GitHub-hosted workflows
			workflows := make([]githubapi.Workflow, 0, len(allWorkflows))
			for _, wf := range allWorkflows {
				if strings.HasPrefix(wf.Path, "dynamic") {
					continue
				}
				workflows = append(workflows, wf)
			}

			selected, err := filterWorkflows(workflows, mustGetStringSlice(flagWorkflow))
			if err != nil {
				return err
			}
			if len(selected) == 0 {
				fmt.Fprintf(stderr, "No workflows in %s/%s matched the current selection.\n", owner, repo)
				return nil
			}

			createdFilter := fmt.Sprintf("%s..%s", from.Format(time.RFC3339), to.Format(time.RFC3339))
			runFilter := githubapi.WorkflowRunFilter{
				Branch:  viper.GetString(flagBranch),
				Status:  viper.GetString(flagStatus),
				Created: createdFilter,
			}

			runLimit, err := cmd.Flags().GetInt(flagSummaryRuns)
			if err != nil {
				return err
			}
			if runLimit < 0 {
				return fmt.Errorf("--%s must be greater than or equal to 0", flagSummaryRuns)
			}
			if runLimit > 0 {
				runFilter.Created = ""
			}

			threads := viper.GetInt(flagThreads)
			if threads <= 0 {
				threads = 1
			}

			var (
				mu      sync.Mutex
				records []metrics.RunRecord
			)

			// Start spinner for fetching workflow runs
			terminal := term.FromEnv()
			var s *spinner.Spinner
			if terminal.IsTerminalOutput() {
				s = spinner.New(spinner.CharSets[11], 100*time.Millisecond)
				s.Suffix = fmt.Sprintf(" Fetching workflow runs for %d workflows...", len(selected))
				s.Start()
			}

			sem := semaphore.NewWeighted(int64(threads))
			g, gctx := errgroup.WithContext(ctx)

			for _, wf := range selected {
				workflow := wf
				g.Go(func() error {
					if err := sem.Acquire(gctx, 1); err != nil {
						return err
					}
					defer sem.Release(1)

					runs, err := client.ListWorkflowRuns(gctx, owner, repo, workflow.ID, runFilter, runLimit)
					if err != nil {
						return fmt.Errorf("workflow %s: %w", workflow.Name, err)
					}

					for _, run := range runs {
						jobs, jobErr := client.ListJobs(gctx, owner, repo, run.ID)
						if jobErr != nil {
							slog.Warn("failed to fetch jobs", slog.String("workflow", workflow.Name), slog.Int64("run", run.ID), slog.String("error", jobErr.Error()))
						}
						mu.Lock()
						records = append(records, metrics.RunRecord{Workflow: workflow, Run: run, Jobs: jobs})
						mu.Unlock()
					}
					return nil
				})
			}

			err = g.Wait()
			if s != nil {
				s.Stop()
			}
			if err != nil {
				return err
			}

			if runLimit > 0 && len(records) > 0 {
				var earliest, latest time.Time
				for _, rec := range records {
					runTime := rec.Run.RunStartedAt
					if runTime.IsZero() {
						runTime = rec.Run.CreatedAt
					}
					if runTime.IsZero() {
						continue
					}
					if earliest.IsZero() || runTime.Before(earliest) {
						earliest = runTime
					}
					if latest.IsZero() || runTime.After(latest) {
						latest = runTime
					}
				}
				if !earliest.IsZero() {
					from = earliest
				} else {
					from = time.Time{}
				}
				if !latest.IsZero() {
					to = latest
				} else {
					to = time.Now().UTC()
				}
				if to.Before(from) {
					to = from
				}
			}

			summary := metrics.Aggregate(records, from, to)

			if viper.GetBool(flagJSON) {
				encoder := json.NewEncoder(stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(summary)
			}

			if csvPath := strings.TrimSpace(viper.GetString(flagCSV)); csvPath != "" {
				if err := writeCSV(summary, csvPath); err != nil {
					return err
				}
			}

			if viper.GetBool(flagMarkdown) {
				renderMarkdownSummary(stdout, summary)
				return nil
			}

			// Pretty colored output
			terminal2 := term.FromEnv()
			renderColoredSummary(os.Stdout, summary, terminal2.IsColorEnabled())
			return nil
		},
	}

	cmd.Flags().Int(flagSummaryRuns, 0, "Fetch only the most recent N runs per workflow (overrides time range filters)")

	return cmd
}

func filterWorkflows(workflows []githubapi.Workflow, selectors []string) ([]githubapi.Workflow, error) {
	if len(workflows) == 0 {
		return nil, fmt.Errorf("repository has no workflows")
	}
	if len(selectors) == 0 {
		return workflows, nil
	}

	matched := make([]githubapi.Workflow, 0, len(selectors))
	for _, wf := range workflows {
		if workflowMatches(wf, selectors) {
			matched = append(matched, wf)
		}
	}
	return matched, nil
}

func workflowMatches(workflow githubapi.Workflow, selectors []string) bool {
	for _, raw := range selectors {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			continue
		}
		if strings.EqualFold(candidate, workflow.Name) {
			return true
		}
		if strings.EqualFold(candidate, workflow.Path) {
			return true
		}
		if strings.EqualFold(candidate, filepath.Base(workflow.Path)) {
			return true
		}
		if id, err := strconv.ParseInt(candidate, 10, 64); err == nil && id == workflow.ID {
			return true
		}
	}
	return false
}

func writeCSV(rows []metrics.SummaryRow, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create csv file: %w", err)
	}
	defer file.Close()

	return output.WriteSummaryCSV(file, rows)
}

func renderColoredSummary(w io.Writer, rows []metrics.SummaryRow, colorEnabled bool) {
	if !colorEnabled {
		color.NoColor = true
	}

	// Title
	titleColor := color.New(color.FgCyan, color.Bold)
	fmt.Fprintln(w)
	titleColor.Fprintln(w, "📊 Workflow Execution Summary")
	fmt.Fprintln(w)

	if len(rows) == 0 {
		warningColor := color.New(color.FgYellow)
		warningColor.Fprintln(w, "⚠️  No workflow runs found in the specified time range")
		return
	}

	// Create table
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Workflow", "Runs", "Failed", "Failure Rate", "Avg Duration", "Total Duration", "Top Runners"})
	table.SetBorder(true)
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
	)
	table.SetColumnColor(
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiWhiteColor},
		tablewriter.Colors{tablewriter.FgGreenColor},
		tablewriter.Colors{tablewriter.FgRedColor},
		tablewriter.Colors{tablewriter.FgYellowColor},
		tablewriter.Colors{tablewriter.FgBlueColor},
		tablewriter.Colors{tablewriter.FgMagentaColor},
		tablewriter.Colors{tablewriter.FgHiBlackColor},
	)

	for _, row := range rows {
		failureRate := fmt.Sprintf("%.1f%%", row.FailureRate*100)
		avgDuration := output.FormatDuration(row.AvgDuration)
		totalDuration := output.FormatDuration(row.TotalDuration)
		topRunners := output.FormatRunnerSummary(row.RunnerSummary, 2)

		table.Append([]string{
			row.Workflow,
			fmt.Sprintf("%d", row.Runs),
			fmt.Sprintf("%d", row.Failed),
			failureRate,
			avgDuration,
			totalDuration,
			topRunners,
		})
	}

	table.Render()
	fmt.Fprintln(w)

	for _, row := range rows {
		if len(row.Jobs) == 0 {
			continue
		}

		jobTitle := color.New(color.FgHiWhite, color.Bold)
		jobTitle.Fprintf(w, "🔧 Jobs for %s\n", row.Workflow)

		jobTable := tablewriter.NewWriter(w)
		jobTable.SetHeader([]string{"Job", "Runs", "Failed", "Failure Rate", "Avg Duration", "Total Duration", "Top Runners"})
		jobTable.SetBorder(true)
		jobTable.SetHeaderColor(
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		)
		jobTable.SetColumnColor(
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiWhiteColor},
			tablewriter.Colors{tablewriter.FgGreenColor},
			tablewriter.Colors{tablewriter.FgRedColor},
			tablewriter.Colors{tablewriter.FgYellowColor},
			tablewriter.Colors{tablewriter.FgBlueColor},
			tablewriter.Colors{tablewriter.FgMagentaColor},
			tablewriter.Colors{tablewriter.FgHiBlackColor},
		)

		for _, job := range row.Jobs {
			jobFailureRate := fmt.Sprintf("%.1f%%", job.FailureRate*100)
			jobAvgDuration := output.FormatDuration(job.AvgDuration)
			jobTotalDuration := output.FormatDuration(job.TotalDuration)
			jobTopRunners := output.FormatRunnerSummary(job.RunnerSummary, 2)

			jobTable.Append([]string{
				job.Job,
				fmt.Sprintf("%d", job.Runs),
				fmt.Sprintf("%d", job.Failed),
				jobFailureRate,
				jobAvgDuration,
				jobTotalDuration,
				jobTopRunners,
			})
		}

		jobTable.Render()
		fmt.Fprintln(w)
	}
}

func renderMarkdownSummary(w io.Writer, rows []metrics.SummaryRow) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "# Workflow Execution Summary")
	fmt.Fprintln(w)

	if len(rows) == 0 {
		fmt.Fprintln(w, "_No workflow runs found in the specified time range._")
		return
	}

	fmt.Fprintln(w, "| Workflow | Runs | Failed | Failure Rate | Avg Duration | Total Duration | Top Runners |")
	fmt.Fprintln(w, "| --- | ---: | ---: | ---: | ---: | ---: | --- |")
	for _, row := range rows {
		failureRate := output.FormatFailureRate(row.FailureRate)
		avgDuration := output.FormatDuration(row.AvgDuration)
		totalDuration := output.FormatDuration(row.TotalDuration)
		topRunners := output.FormatRunnerSummary(row.RunnerSummary, len(row.RunnerSummary))

		fmt.Fprintf(w, "| %s | %d | %d | %s | %s | %s | %s |\n",
			row.Workflow,
			row.Runs,
			row.Failed,
			failureRate,
			avgDuration,
			totalDuration,
			topRunners,
		)
	}
	fmt.Fprintln(w)

	for _, row := range rows {
		if len(row.Jobs) == 0 {
			continue
		}

		fmt.Fprintf(w, "## Jobs for %s\n\n", row.Workflow)
		fmt.Fprintln(w, "| Job | Runs | Failed | Failure Rate | Avg Duration | Total Duration | Top Runners |")
		fmt.Fprintln(w, "| --- | ---: | ---: | ---: | ---: | ---: | --- |")
		for _, job := range row.Jobs {
			failureRate := output.FormatFailureRate(job.FailureRate)
			avgDuration := output.FormatDuration(job.AvgDuration)
			totalDuration := output.FormatDuration(job.TotalDuration)
			topRunners := output.FormatRunnerSummary(job.RunnerSummary, len(job.RunnerSummary))

			fmt.Fprintf(w, "| %s | %d | %d | %s | %s | %s | %s |\n",
				job.Job,
				job.Runs,
				job.Failed,
				failureRate,
				avgDuration,
				totalDuration,
				topRunners,
			)
		}
		fmt.Fprintln(w)
	}
}

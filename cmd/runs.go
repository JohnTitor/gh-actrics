package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/JohnTitor/gh-actrics/internal/githubapi"
	"github.com/JohnTitor/gh-actrics/internal/util"
	"github.com/briandowns/spinner"
	"github.com/cli/go-gh/v2/pkg/tableprinter"
	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newRunsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs <owner>/<repo>",
		Short: "List workflow run details",
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
				return err
			}

			allWorkflows, err := client.ListWorkflows(ctx, owner, repo)
			if err != nil {
				return fmt.Errorf("failed to list workflows for %s/%s: %w", owner, repo, err)
			}

			// Filter out GitHub-hosted dependabot workflows
			workflows := make([]githubapi.Workflow, 0, len(allWorkflows))
			for _, wf := range allWorkflows {
				if wf.Path == "dynamic/dependabot/dependabot-updates" {
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

			limit, err := cmd.Flags().GetInt("limit")
			if err != nil {
				return err
			}
			if limit <= 0 {
				limit = 50
			}

			rows := make([]runRow, 0)

			terminal := term.FromEnv()
			var s *spinner.Spinner
			if terminal.IsTerminalOutput() {
				s = spinner.New(spinner.CharSets[11], 100*time.Millisecond)
				s.Suffix = fmt.Sprintf(" Fetching runs for %d workflows...", len(selected))
				s.Start()
			}

			for _, workflow := range selected {
				runs, err := client.ListWorkflowRuns(ctx, owner, repo, workflow.ID, runFilter)
				if err != nil {
					if s != nil {
						s.Stop()
					}
					return fmt.Errorf("workflow %s: %w", workflow.Name, err)
				}
				for _, run := range runs {
					rows = append(rows, runRow{
						WorkflowID:   workflow.ID,
						WorkflowName: workflow.Name,
						RunID:        run.ID,
						Status:       run.Status,
						Conclusion:   run.Conclusion,
						CreatedAt:    run.CreatedAt,
						UpdatedAt:    run.UpdatedAt,
						Duration:     run.Duration,
						RunAttempt:   run.RunAttempt,
						RunNumber:    run.RunNumber,
						HeadBranch:   run.HeadBranch,
					})
					if limit > 0 && countRuns(rows, workflow.ID) >= limit {
						break
					}
				}
			}
			if s != nil {
				s.Stop()
			}

			if viper.GetBool(flagJSON) {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(rows)
			}

			if csvPath := viper.GetString(flagCSV); csvPath != "" {
				if err := exportRunsCSV(rows, csvPath); err != nil {
					return err
				}
				return nil
			}

			isTerminal := isTerminalWriter(os.Stdout)
			terminalWidth := 120
			tp := tableprinter.New(os.Stdout, isTerminal, terminalWidth)
			tp.AddHeader([]string{"Workflow", "RunID", "Status", "Conclusion", "Duration", "Branch", "Run#", "Attempt", "Created"})
			for _, row := range rows {
				tp.AddField(row.WorkflowName)
				tp.AddField(strconv.FormatInt(row.RunID, 10))
				tp.AddField(row.Status)
				tp.AddField(row.Conclusion)
				tp.AddField(row.Duration.Truncate(time.Second).String())
				tp.AddField(row.HeadBranch)
				tp.AddField(strconv.Itoa(row.RunNumber))
				tp.AddField(strconv.Itoa(row.RunAttempt))
				tp.AddField(row.CreatedAt.Format(time.RFC3339))
				tp.EndRow()
			}
			if len(rows) == 0 {
				tp.AddField("(no runs)")
				tp.EndRow()
			}
			if err := tp.Render(); err != nil {
				return fmt.Errorf("failed to render table: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().Int("limit", 50, "Maximum number of runs to fetch per workflow")
	return cmd
}

type runRow struct {
	WorkflowID   int64         `json:"workflow_id"`
	WorkflowName string        `json:"workflow_name"`
	RunID        int64         `json:"run_id"`
	Status       string        `json:"status"`
	Conclusion   string        `json:"conclusion"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	Duration     time.Duration `json:"duration"`
	RunAttempt   int           `json:"run_attempt"`
	RunNumber    int           `json:"run_number"`
	HeadBranch   string        `json:"head_branch"`
}

func countRuns(rows []runRow, workflowID int64) int {
	count := 0
	for i := range rows {
		if rows[i].WorkflowID == workflowID {
			count++
		}
	}
	return count
}

func exportRunsCSV(rows []runRow, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	header := []string{"workflow_id", "workflow_name", "run_id", "status", "conclusion", "created_at", "updated_at", "duration_ms", "run_number", "run_attempt", "branch"}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, row := range rows {
		record := []string{
			strconv.FormatInt(row.WorkflowID, 10),
			row.WorkflowName,
			strconv.FormatInt(row.RunID, 10),
			row.Status,
			row.Conclusion,
			row.CreatedAt.Format(time.RFC3339),
			row.UpdatedAt.Format(time.RFC3339),
			fmt.Sprintf("%d", row.Duration.Milliseconds()),
			strconv.Itoa(row.RunNumber),
			strconv.Itoa(row.RunAttempt),
			row.HeadBranch,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}

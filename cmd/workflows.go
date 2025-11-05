package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/JohnTitor/gh-actrics/internal/githubapi"
	"github.com/JohnTitor/gh-actrics/internal/util"
	"github.com/briandowns/spinner"
	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newWorkflowsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflows <owner>/<repo>",
		Short: "Display workflows for a repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			owner, repo, err := util.ParseRepo(args[0])
			if err != nil {
				return err
			}

			cacheTTL := viper.GetDuration(flagCacheTTL)
			enableCache := cacheTTL > 0 && !viper.GetBool(flagNoCache)

			client, err := githubapi.NewClient(githubapi.Options{CacheTTL: cacheTTL, EnableCache: enableCache})
			if err != nil {
				return err
			}

			terminal := term.FromEnv()
			var s *spinner.Spinner
			if terminal.IsTerminalOutput() {
				s = spinner.New(spinner.CharSets[11], 100*time.Millisecond)
				s.Suffix = fmt.Sprintf(" Fetching workflows for %s/%s...", owner, repo)
				s.Start()
			}

			allWorkflows, err := client.ListWorkflows(ctx, owner, repo)
			if s != nil {
				s.Stop()
			}
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

			if len(workflows) == 0 {
				fmt.Fprintf(stderr, "No workflows found for %s/%s.\n", owner, repo)
				return nil
			}

			if viper.GetBool(flagJSON) {
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(workflows)
			}

			// Pretty colored output
			terminal2 := term.FromEnv()
			renderColoredWorkflows(os.Stdout, workflows, owner, repo, terminal2.IsColorEnabled())
			return nil
		},
	}

	return cmd
}

func renderColoredWorkflows(w io.Writer, workflows []githubapi.Workflow, owner, repo string, colorEnabled bool) {
	if !colorEnabled {
		color.NoColor = true
	}

	// Title
	titleColor := color.New(color.FgCyan, color.Bold)
	fmt.Fprintln(w)
	titleColor.Fprintf(w, "⚙️  Workflows in %s/%s\n", owner, repo)
	fmt.Fprintln(w)

	// Create table
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"ID", "Name", "Path", "State"})
	table.SetBorder(true)
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
	)
	table.SetColumnColor(
		tablewriter.Colors{tablewriter.FgHiBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiWhiteColor},
		tablewriter.Colors{tablewriter.FgBlueColor},
		tablewriter.Colors{tablewriter.FgGreenColor},
	)

	for _, workflow := range workflows {
		state := workflow.State
		if state == "active" {
			state = "✓ active"
		}
		table.Append([]string{
			fmt.Sprintf("%d", workflow.ID),
			workflow.Name,
			workflow.Path,
			state,
		})
	}

	table.Render()
	fmt.Fprintln(w)
}

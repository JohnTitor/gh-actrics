package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	flagFrom     = "from"
	flagTo       = "to"
	flagLast     = "last"
	flagWorkflow = "workflow"
	flagBranch   = "branch"
	flagStatus   = "status"
	flagJSON     = "json"
	flagCSV      = "csv"
	flagThreads  = "threads"
	flagCacheTTL = "cache-ttl"
	flagNoCache  = "no-cache"
	flagLogLevel = "log-level"
	defaultLast  = "30d"
)

var (
	rootCmd *cobra.Command
	stdout  io.Writer
	stderr  io.Writer
)

// Execute runs the CLI.
func Execute(ctx context.Context, out io.Writer, errOut io.Writer) error {
	stdout = out
	stderr = errOut

	if rootCmd == nil {
		rootCmd = newRootCmd()
	}

	rootCmd.SetOut(out)
	rootCmd.SetErr(errOut)
	return rootCmd.ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gh-actrics",
		Short: "Aggregate and visualize GitHub Actions workflow execution metrics",
		Long: heredoc.Doc(`
			Calculate average/total duration, failure rate, and runner usage for GitHub Actions workflow runs over a customizable time period,
			then present the results in a colorful CLI table or export them for further analysis.
		`),
		SilenceErrors: false,
		SilenceUsage:  false,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := bindPersistentFlags(cmd); err != nil {
				return err
			}
			level := parseLogLevel(viper.GetString(flagLogLevel))
			handler := slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: level})
			slog.SetDefault(slog.New(handler))
			return nil
		},
	}

	cmd.PersistentFlags().String(flagFrom, "", "Start of the reporting window (RFC3339)")
	cmd.PersistentFlags().String(flagTo, "", "End of the reporting window (RFC3339)")
	cmd.PersistentFlags().String(flagLast, defaultLast, "Length of the look-back window (e.g. 7d, 4w, 3mo)")
	cmd.PersistentFlags().StringSlice(flagWorkflow, nil, "Target workflows (IDs, filenames, or names; repeatable)")
	cmd.PersistentFlags().String(flagBranch, "", "Filter runs by branch")
	cmd.PersistentFlags().String(flagStatus, "", "Filter runs by combined status (success, failure, cancelled, etc.)")
	cmd.PersistentFlags().Bool(flagJSON, false, "Print aggregated metrics as JSON")
	cmd.PersistentFlags().String(flagCSV, "", "Write aggregated metrics as CSV to the given path")
	cmd.PersistentFlags().Int(flagThreads, 4, "Maximum number of concurrent API requests")
	cmd.PersistentFlags().Duration(flagCacheTTL, 0, "Duration to cache API responses (e.g. 10m, 1h)")
	cmd.PersistentFlags().Bool(flagNoCache, false, "Disable on-disk API response cache")
	cmd.PersistentFlags().String(flagLogLevel, "info", "Minimum log level (debug|info|warn|error)")

	viper.SetEnvPrefix("GH_ACTIONS_METRICS")
	viper.AutomaticEnv()
	if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
		fmt.Fprintf(os.Stderr, "failed to bind flags: %v\n", err)
	}

	cmd.AddCommand(newSummaryCmd())
	cmd.AddCommand(newWorkflowsCmd())
	cmd.AddCommand(newRunsCmd())

	return cmd
}

func bindPersistentFlags(cmd *cobra.Command) error {
	return viper.BindPFlags(cmd.Flags())
}

func isTerminalWriter(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(f)
	}
	return false
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func resolveTimeRange(now time.Time, fromStr, toStr, last string) (time.Time, time.Time, error) {
	var (
		from time.Time
		to   time.Time
		err  error
	)

	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --to value: %w", err)
		}
	} else {
		to = now
	}

	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --from value: %w", err)
		}
	} else {
		d, err := parseLastDuration(last)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		from = to.Add(-d)
	}

	if !from.Before(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("--from must be before --to")
	}

	return from, to, nil
}

func parseLastDuration(input string) (time.Duration, error) {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		input = defaultLast
	}
	if d, err := time.ParseDuration(input); err == nil {
		return d, nil
	}

	if strings.HasSuffix(input, "d") {
		days := strings.TrimSuffix(input, "d")
		return parseScalarDuration(days, 24*time.Hour)
	}
	if strings.HasSuffix(input, "w") {
		weeks := strings.TrimSuffix(input, "w")
		return parseScalarDuration(weeks, 7*24*time.Hour)
	}
	if strings.HasSuffix(input, "mo") {
		months := strings.TrimSuffix(input, "mo")
		return parseScalarDuration(months, 30*24*time.Hour)
	}
	return 0, fmt.Errorf("invalid --last format: %s", input)
}

func parseScalarDuration(value string, unit time.Duration) (time.Duration, error) {
	var amount float64
	if _, err := fmt.Sscanf(value, "%f", &amount); err != nil {
		return 0, fmt.Errorf("invalid duration %s: %w", value, err)
	}
	if amount <= 0 {
		return 0, fmt.Errorf("duration must be > 0")
	}
	return time.Duration(amount * float64(unit)), nil
}

func mustGetStringSlice(flag string) []string {
	values := viper.GetStringSlice(flag)
	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

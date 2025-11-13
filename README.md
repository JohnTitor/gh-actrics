# gh-actrics

> 📊 A GitHub CLI extension to aggregate and visualize GitHub Actions workflow execution metrics

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Overview

`gh-actrics` (GitHub Actions Metrics) is a powerful CLI tool that helps you analyze and understand your GitHub Actions workflow performance. It aggregates execution metrics across workflows, providing insights into duration, failure rates, runner usage, and more.

Perfect for DevOps teams, Platform Engineers, and SREs who need to monitor and optimize their CI/CD pipelines.

## Features

- Average and total execution duration per workflow
- Failure rates and success tracking
- Runner usage statistics (labels, execution time)
- Execution counts over customizable time periods
- JSON, CSV, and Markdown output support

## Installation

### Via GitHub CLI

```bash
gh extension install JohnTitor/gh-actrics
```

### From Source

```bash
git clone https://github.com/JohnTitor/gh-actrics.git
cd gh-actrics
go build
gh extension install .
```

## Configuration

### Authentication

`gh-actrics` uses GitHub CLI's authentication. Ensure you're logged in:

```bash
gh auth login
```

Required scopes:
- `repo` - Access repository data
- `actions:read` - Read Actions data

### Environment Variables

All flags can be set via environment variables with the `GH_ACTIONS_METRICS_` prefix:

```bash
export GH_ACTIONS_METRICS_THREADS=8
export GH_ACTIONS_METRICS_CACHE_TTL=15m
export GH_ACTIONS_METRICS_LOG_LEVEL=debug

gh actrics summary owner/repo
```

### Response Caching

When `--cache-ttl` is set to a positive duration (or `GH_ACTIONS_METRICS_CACHE_TTL` is configured), `gh-actrics` persists GitHub API responses in `~/.cache/gh-actrics`. Repeated invocations within the TTL reuse these cached payloads to reduce rate-limit pressure. Use `--no-cache` (or `GH_ACTIONS_METRICS_NO_CACHE=true`) to bypass the cache when fresh data is required.

## Usage

### Quick Start

Get a summary of all workflows in the last 30 days:

```bash
gh actrics summary owner/repo
```

### Commands

#### `summary` - Aggregate Workflow Metrics

Analyze workflow execution metrics over a time period.

```bash
gh actrics summary owner/repo [flags]
```

**Example Output:**
```
📊 Workflow Execution Summary

┌──────────────┬──────┬────────┬──────────────┬──────────────┬────────────────┬─────────────────────┐
│   WORKFLOW   │ RUNS │ FAILED │ FAILURE RATE │ AVG DURATION │ TOTAL DURATION │    TOP RUNNERS      │
├──────────────┼──────┼────────┼──────────────┼──────────────┼────────────────┼─────────────────────┤
│ CI           │   45 │      3 │      6.7%    │    5m23s     │    4h1m        │ ubuntu-latest(45)   │
│ Deploy       │   12 │      1 │      8.3%    │    12m45s    │    2h33m       │ self-hosted(12)     │
└──────────────┴──────┴────────┴──────────────┴──────────────┴────────────────┴─────────────────────┘
```

Limit aggregation to the latest runs:

```bash
gh actrics summary owner/repo --runs 10
```

This collects only the ten most recent runs per workflow and ignores any time range flags.

#### `workflows` - List Repository Workflows

Display all workflows in a repository.

```bash
gh actrics workflows owner/repo [flags]
```

**Example Output:**
```
⚙️  Workflows in owner/repo

┌──────────┬───────────────┬──────────────────────────────┬───────────┐
│    ID    │     NAME      │            PATH              │   STATE   │
├──────────┼───────────────┼──────────────────────────────┼───────────┤
│ 12345678 │ CI            │ .github/workflows/ci.yml     │ ✓ active  │
│ 12345679 │ Deploy        │ .github/workflows/deploy.yml │ ✓ active  │
└──────────┴───────────────┴──────────────────────────────┴───────────┘
```

#### `runs` - List Workflow Runs

Display detailed information about workflow runs.

```bash
gh actrics runs owner/repo [flags]
```

### Common Flags

#### Time Range

```bash
# Last 7 days (default: 30d)
gh actrics summary owner/repo --last 7d

# Specific date range
gh actrics summary owner/repo --from 2025-01-01 --to 2025-01-31
```

#### Workflow Filtering

```bash
# Specific workflow by name
gh actrics summary owner/repo --workflow "CI"

# Multiple workflows
gh actrics summary owner/repo --workflow "CI" --workflow "Deploy"

# By filename
gh actrics summary owner/repo --workflow "ci.yml"
```

#### Branch Filtering

```bash
# Only main branch
gh actrics summary owner/repo --branch main
```

#### Latest Runs

```bash
# Most recent 10 runs per workflow
gh actrics summary owner/repo --runs 10
```

`--runs` ignores `--from`, `--to`, and `--last`, fetching the newest runs directly from the GitHub API.

#### Output Formats

```bash
# JSON output
gh actrics summary owner/repo --json

# CSV export
gh actrics summary owner/repo --csv metrics.csv

# Markdown output
gh actrics summary owner/repo --markdown
```

### Complete Flag Reference

| Flag | Description | Default |
|------|-------------|---------|
| `--from` | Start of reporting window (RFC3339) | - |
| `--to` | End of reporting window (RFC3339) | - |
| `--last` | Look-back window (e.g., 7d, 4w, 3mo) | `30d` |
| `--workflow` | Target workflows (repeatable) | All |
| `--branch` | Filter by branch | All |
| `--status` | Filter by status | All |
| `--runs` | Fetch only the most recent N runs per workflow (overrides time range filters) | `0` (disabled) |
| `--json` | JSON output | `false` |
| `--csv` | Write CSV to path | - |
| `--markdown` | Render Markdown tables to stdout | `false` |
| `--threads` | Concurrent API requests | `4` |
| `--cache-ttl` | Cache duration (e.g., 10m, 1h) | `0` |
| `--no-cache` | Disable cache | `false` |
| `--log-level` | Logging level (debug/info/warn/error) | `info` |

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details

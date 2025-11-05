package githubapi

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Options defines configuration for the GitHub API client.
type Options struct {
	CacheTTL    time.Duration
	EnableCache bool
}

// Client wraps github.com/cli/go-gh REST client for higher-level operations.
type Client struct {
	rest *api.RESTClient
}

// NewClient constructs a Client respecting gh configuration.
func NewClient(opts Options) (*Client, error) {
	clientOpts := api.ClientOptions{}
	if opts.CacheTTL > 0 && opts.EnableCache {
		clientOpts.CacheTTL = opts.CacheTTL
		clientOpts.EnableCache = true
	}

	rest, err := api.NewRESTClient(clientOpts)
	if err != nil {
		return nil, err
	}
	return &Client{rest: rest}, nil
}

// ListWorkflows returns all workflows in the repository.
func (c *Client) ListWorkflows(ctx context.Context, owner, repo string) ([]Workflow, error) {
	_ = ctx // context not yet used by go-gh REST client Get
	page := 1
	var workflows []Workflow

	for {
		path := fmt.Sprintf("repos/%s/%s/actions/workflows?per_page=100&page=%d", owner, repo, page)
		var response workflowListResponse
		if err := c.rest.Get(path, &response); err != nil {
			return nil, fmt.Errorf("GET %s: %w", path, err)
		}

		for _, wf := range response.Workflows {
			workflows = append(workflows, Workflow{
				ID:    wf.ID,
				Name:  wf.Name,
				Path:  wf.Path,
				State: wf.State,
			})
		}

		if len(response.Workflows) == 0 || len(workflows) >= response.TotalCount {
			break
		}
		page++
	}

	return workflows, nil
}

// WorkflowRunFilter describes filters for listing workflow runs.
type WorkflowRunFilter struct {
	Branch             string
	Status             string
	Created            string
}

// ListWorkflowRuns returns runs for the given workflow id.
func (c *Client) ListWorkflowRuns(ctx context.Context, owner, repo string, workflowID int64, filter WorkflowRunFilter) ([]WorkflowRun, error) {
	_ = ctx
	page := 1
	var runs []WorkflowRun

	for {
		params := url.Values{}
		params.Set("per_page", "100")
		params.Set("page", strconv.Itoa(page))
		if filter.Branch != "" {
			params.Set("branch", filter.Branch)
		}
		if filter.Status != "" {
			params.Set("status", filter.Status)
		}
		if filter.Created != "" {
			params.Set("created", filter.Created)
		}
		path := fmt.Sprintf("repos/%s/%s/actions/workflows/%d/runs?%s", owner, repo, workflowID, params.Encode())

		var response workflowRunsResponse
		if err := c.rest.Get(path, &response); err != nil {
			return nil, fmt.Errorf("GET %s: %w", path, err)
		}

		for _, run := range response.WorkflowRuns {
			runs = append(runs, mapWorkflowRun(run))
		}

		if len(response.WorkflowRuns) == 0 || len(runs) >= response.TotalCount {
			break
		}
		page++
	}

	return runs, nil
}

// ListJobs returns jobs for a workflow run.
func (c *Client) ListJobs(ctx context.Context, owner, repo string, runID int64) ([]WorkflowJob, error) {
	_ = ctx
	page := 1
	var jobs []WorkflowJob

	for {
		path := fmt.Sprintf("repos/%s/%s/actions/runs/%d/jobs?per_page=100&page=%d", owner, repo, runID, page)
		var response workflowJobsResponse
		if err := c.rest.Get(path, &response); err != nil {
			return nil, fmt.Errorf("GET %s: %w", path, err)
		}

		for _, job := range response.Jobs {
			jobs = append(jobs, mapWorkflowJob(job))
		}

		if len(response.Jobs) == 0 || len(jobs) >= response.TotalCount {
			break
		}
		page++
	}

	return jobs, nil
}

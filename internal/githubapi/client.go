package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/JohnTitor/gh-actrics/internal/cache"
	"github.com/cli/go-gh/v2/pkg/api"
)

// Options defines configuration for the GitHub API client.
type Options struct {
	CacheTTL    time.Duration
	EnableCache bool
	CacheDir    string
}

type restClient interface {
	Get(path string, response interface{}) error
}

// Client wraps github.com/cli/go-gh REST client for higher-level operations.
type Client struct {
	rest  restClient
	cache *cache.Cache
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

	var cacheStore *cache.Cache
	if opts.EnableCache && opts.CacheTTL > 0 {
		cacheDir := opts.CacheDir
		if cacheDir == "" {
			cacheDir, err = defaultCacheDir()
			if err != nil {
				return nil, err
			}
		}
		cacheStore, err = cache.New(cacheDir, opts.CacheTTL)
		if err != nil {
			return nil, err
		}
	}

	return &Client{
		rest:  rest,
		cache: cacheStore,
	}, nil
}

// ListWorkflows returns all workflows in the repository.
func (c *Client) ListWorkflows(ctx context.Context, owner, repo string) ([]Workflow, error) {
	_ = ctx // context not yet used by go-gh REST client Get
	page := 1
	var workflows []Workflow

	for {
		path := fmt.Sprintf("repos/%s/%s/actions/workflows?per_page=100&page=%d", owner, repo, page)
		var response workflowListResponse
		if err := c.cachedGet(path, &response); err != nil {
			return nil, err
		}

		for _, wf := range response.Workflows {
			workflows = append(workflows, Workflow(wf))
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
	Branch  string
	Status  string
	Created string
}

// ListWorkflowRuns returns runs for the given workflow id.
func (c *Client) ListWorkflowRuns(ctx context.Context, owner, repo string, workflowID int64, filter WorkflowRunFilter, limit int) ([]WorkflowRun, error) {
	_ = ctx
	page := 1
	var runs []WorkflowRun

	for {
		params := url.Values{}
		perPage := 100
		if limit > 0 {
			remaining := limit - len(runs)
			if remaining <= 0 {
				break
			}
			if remaining < perPage {
				perPage = remaining
			}
		}
		params.Set("per_page", strconv.Itoa(perPage))
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
		if err := c.cachedGet(path, &response); err != nil {
			return nil, err
		}

		for _, run := range response.WorkflowRuns {
			runs = append(runs, mapWorkflowRun(run))
			if limit > 0 && len(runs) >= limit {
				break
			}
		}

		if len(response.WorkflowRuns) == 0 || len(runs) >= response.TotalCount || (limit > 0 && len(runs) >= limit) {
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
		if err := c.cachedGet(path, &response); err != nil {
			return nil, err
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

func (c *Client) cachedGet(path string, out interface{}) error {
	if c.cache != nil {
		if data, ok, err := c.cache.Get(path); err == nil && ok {
			if err := json.Unmarshal(data, out); err == nil {
				return nil
			}
		}
	}

	if err := c.rest.Get(path, out); err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}

	if c.cache != nil {
		if data, err := json.Marshal(out); err == nil {
			_ = c.cache.Set(path, data)
		}
	}
	return nil
}

func defaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "gh-actrics"), nil
}

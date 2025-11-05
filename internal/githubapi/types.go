package githubapi

import "time"

type workflowListResponse struct {
	TotalCount int              `json:"total_count"`
	Workflows  []workflowRecord `json:"workflows"`
}

type workflowRecord struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Path  string `json:"path"`
	State string `json:"state"`
}

type workflowRunsResponse struct {
	TotalCount   int               `json:"total_count"`
	WorkflowRuns []workflowRunJSON `json:"workflow_runs"`
}

type workflowRunJSON struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	DisplayTitle    string     `json:"display_title"`
	Event           string     `json:"event"`
	Status          string     `json:"status"`
	Conclusion      string     `json:"conclusion"`
	CreatedAt       time.Time  `json:"created_at"`
	RunStartedAt    *time.Time `json:"run_started_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	RunNumber       int        `json:"run_number"`
	RunAttempt      int        `json:"run_attempt"`
	RunDuration     int64      `json:"run_duration_ms"`
	WorkflowID      int64      `json:"workflow_id"`
	HeadBranch      string     `json:"head_branch"`
	TriggeringActor *struct {
		Login string `json:"login"`
	} `json:"triggering_actor"`
}

type workflowJobsResponse struct {
	TotalCount int               `json:"total_count"`
	Jobs       []workflowJobJSON `json:"jobs"`
}

type workflowJobJSON struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Conclusion  string     `json:"conclusion"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	RunnerName  string     `json:"runner_name"`
	RunnerID    int64      `json:"runner_id"`
	Labels      []string   `json:"labels"`
}

// Workflow is a simplified workflow descriptor.
type Workflow struct {
	ID    int64
	Name  string
	Path  string
	State string
}

// WorkflowRun represents a workflow run.
type WorkflowRun struct {
	ID              int64
	Name            string
	DisplayTitle    string
	Event           string
	Status          string
	Conclusion      string
	CreatedAt       time.Time
	RunStartedAt    time.Time
	UpdatedAt       time.Time
	RunNumber       int
	RunAttempt      int
	Duration        time.Duration
	WorkflowID      int64
	HeadBranch      string
	TriggeringActor string
}

// WorkflowJob represents a job in a workflow run.
type WorkflowJob struct {
	ID          int64
	Name        string
	Status      string
	Conclusion  string
	StartedAt   time.Time
	CompletedAt time.Time
	RunnerName  string
	RunnerID    int64
	Labels      []string
}

// Duration computes the job duration.
func (j WorkflowJob) Duration() time.Duration {
	if j.StartedAt.IsZero() || j.CompletedAt.IsZero() {
		return 0
	}
	if j.CompletedAt.Before(j.StartedAt) {
		return 0
	}
	return j.CompletedAt.Sub(j.StartedAt)
}

func mapWorkflowRun(run workflowRunJSON) WorkflowRun {
	var start time.Time
	if run.RunStartedAt != nil {
		start = run.RunStartedAt.UTC()
	}
	duration := time.Duration(run.RunDuration) * time.Millisecond
	if duration == 0 && !start.IsZero() {
		duration = run.UpdatedAt.Sub(start)
	}

	var triggeringActor string
	if run.TriggeringActor != nil {
		triggeringActor = run.TriggeringActor.Login
	}

	return WorkflowRun{
		ID:              run.ID,
		Name:            run.Name,
		DisplayTitle:    run.DisplayTitle,
		Event:           run.Event,
		Status:          run.Status,
		Conclusion:      run.Conclusion,
		CreatedAt:       run.CreatedAt.UTC(),
		RunStartedAt:    start,
		UpdatedAt:       run.UpdatedAt.UTC(),
		RunNumber:       run.RunNumber,
		RunAttempt:      run.RunAttempt,
		Duration:        duration,
		WorkflowID:      run.WorkflowID,
		HeadBranch:      run.HeadBranch,
		TriggeringActor: triggeringActor,
	}
}

func mapWorkflowJob(job workflowJobJSON) WorkflowJob {
	return WorkflowJob{
		ID:          job.ID,
		Name:        job.Name,
		Status:      job.Status,
		Conclusion:  job.Conclusion,
		StartedAt:   derefTime(job.StartedAt),
		CompletedAt: derefTime(job.CompletedAt),
		RunnerName:  job.RunnerName,
		RunnerID:    job.RunnerID,
		Labels:      append([]string(nil), job.Labels...),
	}
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.UTC()
}

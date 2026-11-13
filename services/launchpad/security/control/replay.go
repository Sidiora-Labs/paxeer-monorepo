package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ReplayProvider is an explicit owner-adapter binding. The indexer's internal
// ReplayRunner is not a control HTTP API: deployments must supply the owner
// adapter and its checkpoint/generation authorization before enabling this path.
type ReplayProvider struct {
	endpoint   *url.URL
	client     *http.Client
	credential string
}
type ReplayJob struct {
	ActionID           string    `json:"actionId"`
	ActionKey          string    `json:"actionKey"`
	Kind               string    `json:"kind"`
	Scope              string    `json:"scope"`
	State              string    `json:"state"`
	PayloadHash        string    `json:"payloadHash"`
	PayloadReference   string    `json:"payloadReference"`
	CompletionIdentity string    `json:"completionIdentity"`
	Claim              string    `json:"claim"`
	Until              time.Time `json:"until"`
}
type ReplayExecution struct {
	JobID   string        `json:"jobId"`
	Job     ReplayJob     `json:"job"`
	Payload ReplayPayload `json:"payload"`
}
type ReplayResult struct{ ActionID, JobID, PayloadHash, PayloadReference, CompletionIdentity, State, Reference string }

func NewReplayProvider(endpoint, credential string, transport *http.Transport) (*ReplayProvider, error) {
	base, err := NewRevocationProvider(endpoint, credential, transport)
	if err != nil {
		return nil, err
	}
	return &ReplayProvider{base.endpoint, base.client, credential}, nil
}
func (p *ReplayProvider) Execute(ctx context.Context, request ReplayExecution) (_ ReplayResult, resultErr error) {
	var result ReplayResult
	if p == nil || p.client == nil || !key(request.JobID) || request.Job.CompletionIdentity != "control-replay:"+request.JobID || digest(request.Payload) != request.Job.PayloadHash {
		return result, Invalid
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return result, err
	}
	u := *p.endpoint
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/projection-replays/" + url.PathEscape(request.Job.CompletionIdentity)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(raw))
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", "Bearer "+p.credential)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", request.Job.CompletionIdentity)
	response, err := p.client.Do(req)
	if err != nil {
		return result, Unavailable
	}
	defer func() { resultErr = errors.Join(resultErr, response.Body.Close()) }()
	if response.StatusCode != http.StatusOK {
		return result, Unavailable
	}
	raw, err = io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return result, Unavailable
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if rejectDuplicateFields(raw) != nil || decoder.Decode(&result) != nil || decoder.Decode(new(any)) != io.EOF {
		return result, Unavailable
	}
	if result.ActionID != request.Job.ActionID || result.JobID != request.JobID || result.PayloadHash != request.Job.PayloadHash || result.PayloadReference != request.Job.PayloadReference || result.CompletionIdentity != request.Job.CompletionIdentity || (result.State != controlSucceeded && result.State != controlFailed) || !text(result.Reference, 256) {
		return ReplayResult{}, Unavailable
	}
	return result, nil
}

type ReplayWorker struct {
	Store      *Store
	Provider   *ReplayProvider
	Credential string
	Observe    func(string, error)
}

func (w ReplayWorker) Process(ctx context.Context, actionKey, scope string) error {
	if w.Store == nil || w.Provider == nil || w.Credential == "" {
		return Invalid
	}
	var action Action
	var job ReplayJob
	var payload ReplayPayload
	var result ReplayResult
	claim := id()
	hasResult := false
	err := w.Store.run(ctx, w.Credential, "action", scope, "claim approved projection replay", func(t *transaction) error {
		if err := t.get("action", actionKey, &action); err != nil {
			return err
		}
		if action.Scope != scope || action.Kind != controlReplay || action.State != controlExecuting || !t.approvalActive(action) || !action.Expires.After(t.now) {
			return Denied
		}
		if err := t.get(controlJob, action.JobID, &job); err != nil {
			return err
		}
		if job.ActionID != action.ID || job.ActionKey != actionKey || job.PayloadReference != action.PayloadReference || job.PayloadHash != action.PayloadHash || job.CompletionIdentity != action.CompletionIdentity || action.CompletionIdentity != "control-replay:"+action.JobID {
			return Conflict
		}
		if err := t.get("action_payload", job.PayloadReference, &payload); err != nil {
			return err
		}
		if digest(payload) != job.PayloadHash {
			return Conflict
		}
		if err := t.get("executor_result", action.JobID, &result); err == nil {
			hasResult = true
			return nil
		} else if err != Missing {
			return err
		}
		var schedule workSchedule
		if err := t.get("work_schedule", "job."+action.JobID, &schedule); err != nil && err != Missing {
			return err
		}
		if schedule.Next.After(t.now) || job.Until.After(t.now) {
			return Unavailable
		}
		if schedule.Attempts < 5 {
			schedule.Attempts++
		}
		delay := 30 * time.Second * time.Duration(1<<(schedule.Attempts-1))
		if delay > 5*time.Minute {
			delay = 5 * time.Minute
		}
		schedule.Last = t.now
		schedule.Next = t.now.Add(delay)
		if err := t.put("work_schedule", "job."+action.JobID, schedule); err != nil {
			return err
		}
		job.Claim = claim
		job.Until = t.now.Add(15 * time.Second)
		job.State = "running"
		return t.put(controlJob, action.JobID, job)
	})
	if err != nil {
		return err
	}
	if !hasResult {
		result, err = w.Provider.Execute(ctx, ReplayExecution{action.JobID, job, payload})
		if err != nil {
			return err
		}
		err = w.Store.run(ctx, w.Credential, "action", scope, "retain owner replay completion", func(t *transaction) error {
			var current ReplayJob
			if err := t.get(controlJob, action.JobID, &current); err != nil {
				return err
			}
			if current.Claim != claim || !current.Until.After(t.now) {
				return Conflict
			}
			current.Claim = ""
			current.Until = time.Time{}
			if err := t.put(controlJob, action.JobID, current); err != nil {
				return err
			}
			return t.put("executor_result", action.JobID, result)
		})
		if err != nil {
			return err
		}
	}
	return w.Store.CompleteAction(ctx, w.Credential, actionKey, scope, "complete verified projection replay", result.State, result.Reference, action.Revision, DurableJobResults{w.Store})
}
func (w ReplayWorker) Drain(ctx context.Context) error {
	if w.Store == nil || w.Provider == nil || w.Credential == "" || w.Observe == nil {
		return Invalid
	}
	items, err := w.Store.pendingWork(ctx, controlJob)
	if err != nil {
		return err
	}
	for _, item := range items {
		var job ReplayJob
		if err = json.Unmarshal(item.Body, &job); err != nil {
			return err
		}
		err = w.Process(ctx, job.ActionKey, job.Scope)
		w.Observe(item.Key, err)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}
func (w ReplayWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		if err := w.Drain(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

package ranker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
	"github.com/google/uuid"
)

var ErrUnavailable = errors.New("ranking unavailable")
var ErrSuperseded = errors.New("ranking completion superseded")
var hashGrammar = regexp.MustCompile(`^[0-9a-f]{64}$`)
var uuidGrammar = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

var heavyCategories = func() []string {
	categories := make([]string, 0, len(Categories)-1)
	for _, category := range Categories {
		if category != categoryNew {
			categories = append(categories, category)
		}
	}
	return categories
}()

type Provenance struct {
	Watermark          v1.Watermark `json:"watermark"`
	UniverseDigest     string       `json:"universeDigest"`
	StatsRevision      string       `json:"statsRevision"`
	ModerationRevision string       `json:"moderationRevision"`
	ActivityRevision   string       `json:"activityRevision"`
	WindowStart        int64        `json:"windowStart"`
	WindowEnd          int64        `json:"windowEnd"`
}

func (p Provenance) Validate(in InputSnapshot) error {
	height, err := strconv.ParseUint(string(p.Watermark.Height), 10, 64)
	observed, e := time.Parse(time.RFC3339Nano, p.Watermark.ObservedAt)
	if err != nil || e != nil || height != in.CutoffHeight || p.Watermark.ChainId != "125" || p.Watermark.Integrity != "healthy" || !regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`).MatchString(p.Watermark.BlockHash) || observed.UTC().Format("2006-01-02T15:04:05.000Z") != p.Watermark.ObservedAt || observed.Unix() <= 0 || observed.Unix() > in.CutoffTimeUTC || !hashGrammar.MatchString(p.UniverseDigest) || p.StatsRevision == "" || len(p.StatsRevision) > 64 || p.ModerationRevision == "" || len(p.ModerationRevision) > 64 || p.ActivityRevision == "" || len(p.ActivityRevision) > 64 || p.WindowEnd != in.CutoffTimeUTC || p.WindowStart < 0 || p.WindowStart >= p.WindowEnd {
		return ErrUnavailable
	}
	revision, err := strconv.ParseUint(string(p.Watermark.ProjectionRevision), 10, 64)
	if err != nil || revision < 1 {
		return ErrUnavailable
	}
	return nil
}

type FrozenSource interface {
	Freeze(context.Context, string, ...time.Time) (InputSnapshot, error)
}

// RefreshRefusal is the safe, structured form of an input refusal. The cause
// is retained for errors.Is checks, while Error deliberately omits its text so
// callers can log only the stable step and class.
type RefreshRefusal struct {
	Step  string
	Class string
	Cause error
}

func (e *RefreshRefusal) Error() string { return "ranking refresh refused" }
func (e *RefreshRefusal) Unwrap() error {
	if e == nil || e.Cause == nil {
		return ErrUnavailable
	}
	return e.Cause
}

func NewRefreshRefusal(step, class string, cause error) error {
	if step == "" {
		step = "unknown"
	}
	if class == "" {
		class = "unavailable"
	}
	return &RefreshRefusal{Step: step, Class: class, Cause: cause}
}

func RefreshRefusalDetails(err error) (step, class string, ok bool) {
	var refusal *RefreshRefusal
	if !errors.As(err, &refusal) || refusal == nil {
		return "", "", false
	}
	return refusal.Step, refusal.Class, true
}

type CompletedSink interface {
	Complete(context.Context, []Snapshot) error
}
type Coordinator struct {
	ObserveOpenFillPools func(newOnly bool, pools int)
	source               any
	sink                 CompletedSink
}

func NewCoordinator(source any, sink CompletedSink) (*Coordinator, error) {
	if source == nil || sink == nil {
		return nil, ErrUnavailable
	}
	return &Coordinator{source: source, sink: sink}, nil
}

type frozenSourceAtDatabaseClock interface {
	FreezeAtDatabaseClock(context.Context, string) (InputSnapshot, error)
}

type frozenSourceWithoutCutoff interface {
	Freeze(context.Context, string) (InputSnapshot, error)
}

type frozenSourceWithCutoff interface {
	Freeze(context.Context, string, time.Time) (InputSnapshot, error)
}

// Refresh accepts the current form (ctx, runID, newOnly) and the legacy form
// (ctx, runID, cutoff, newOnly) used by retained callers. The service loop uses
// the current form, which obtains the cutoff inside the database transaction.
func (c *Coordinator) Refresh(ctx context.Context, runID string, args ...any) ([]Snapshot, error) {
	var cutoff *time.Time
	var newOnly bool
	switch len(args) {
	case 1:
		var ok bool
		newOnly, ok = args[0].(bool)
		if !ok {
			return nil, ErrUnavailable
		}
	case 2:
		value, ok := args[0].(time.Time)
		if !ok {
			return nil, ErrUnavailable
		}
		newOnly, ok = args[1].(bool)
		if !ok {
			return nil, ErrUnavailable
		}
		cutoff = &value
	default:
		return nil, ErrUnavailable
	}
	if c == nil || !uuidGrammar.MatchString(runID) {
		return nil, ErrUnavailable
	}
	in, err := c.freeze(ctx, runID, cutoff)
	if err != nil {
		return nil, err
	}
	if in.RunID != runID {
		return nil, NewRefreshRefusal("identity", "validation", ErrUnavailable)
	}
	if cutoff != nil && in.CutoffTimeUTC != cutoff.Unix() {
		return nil, NewRefreshRefusal("cutoff", "validation", ErrUnavailable)
	}
	if in.CutoffTimeUTC <= 0 || in.Provenance == nil || in.Provenance.Validate(in) != nil {
		return nil, NewRefreshRefusal("provenance", "validation", ErrUnavailable)
	}
	if c.ObserveOpenFillPools != nil {
		c.ObserveOpenFillPools(newOnly, in.OpenFillPools)
	}
	snapshots, err := ComputeRun(in, newOnly)
	if err != nil {
		return nil, err
	}
	if err = c.sink.Complete(ctx, snapshots); err != nil {
		return nil, NewRefreshRefusal("completion", "sink", err)
	}
	return snapshots, nil
}

func (c *Coordinator) freeze(ctx context.Context, runID string, cutoff *time.Time) (InputSnapshot, error) {
	if cutoff == nil {
		if source, ok := c.source.(frozenSourceAtDatabaseClock); ok {
			return source.FreezeAtDatabaseClock(ctx, runID)
		}
		if source, ok := c.source.(frozenSourceWithoutCutoff); ok {
			return source.Freeze(ctx, runID)
		}
		// A legacy source cannot establish the database cutoff. Keep its old
		// path available for explicit callers, but refuse an unbounded run.
		return InputSnapshot{}, ErrUnavailable
	}
	if source, ok := c.source.(frozenSourceWithCutoff); ok {
		return source.Freeze(ctx, runID, *cutoff)
	}
	if source, ok := c.source.(FrozenSource); ok {
		return source.Freeze(ctx, runID, *cutoff)
	}
	return InputSnapshot{}, ErrUnavailable
}
func ComputeRun(in InputSnapshot, newOnly bool) ([]Snapshot, error) {
	if newOnly {
		s, err := BuildSnapshot(in, categoryNew, nil)
		if err != nil {
			return nil, err
		}
		return []Snapshot{s}, nil
	}
	trending, err := BuildSnapshot(in, categoryTrending, nil)
	if err != nil {
		return nil, err
	}
	out := []Snapshot{trending}
	for _, category := range heavyCategories[1:] {
		var dependency *Snapshot
		if category == categoryMovers {
			dependency = &trending
		}
		s, err := buildFrozenSnapshot(trending.input, category, dependency)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}
func (s Snapshot) Input() InputSnapshot {
	b, _ := json.Marshal(s.input)
	var out InputSnapshot
	_ = json.Unmarshal(b, &out)
	return out
}
func (s Snapshot) CutoffTime() int64 { return s.input.CutoffTimeUTC }

type Header struct {
	RunID            string     `json:"runId"`
	Category         string     `json:"category"`
	InputDigest      string     `json:"inputDigest"`
	OutputDigest     string     `json:"outputDigest"`
	AlgorithmVersion string     `json:"algorithmVersion"`
	CutoffTime       int64      `json:"cutoffTime"`
	CompletedAt      time.Time  `json:"completedAt"`
	Revision         int64      `json:"revision"`
	Total            int64      `json:"total"`
	Provenance       Provenance `json:"provenance"`
}

func (h Header) Validate() error {
	if !uuidGrammar.MatchString(h.RunID) || !hashGrammar.MatchString(h.InputDigest) || !hashGrammar.MatchString(h.OutputDigest) || h.AlgorithmVersion != AlgorithmVersion || h.CutoffTime <= 0 || h.CutoffTime > math.MaxInt64-900 || h.CompletedAt.IsZero() || h.Revision < 1 || h.Total < 0 || h.Total > 1000000 {
		return ErrUnavailable
	}
	_, err := Policy(h.Category)
	return err
}
func (h Header) ETag() string {
	p, _ := Policy(h.Category)
	return `"` + digest([]string{h.RunID, h.Category, h.InputDigest, h.OutputDigest, p.Version}) + `"`
}
func (h Header) Stale(now time.Time) bool {
	p, _ := Policy(h.Category)
	return now.Unix() < h.CutoffTime || now.Unix()-h.CutoffTime > p.FreshnessSeconds
}

type RankedItem struct {
	Address string `json:"poolAddress"`
	Score   string `json:"score"`
	Rank    int64  `json:"rank"`
}
type StoredPage struct {
	Header Header       `json:"header"`
	Items  []RankedItem `json:"items"`
	Offset int64        `json:"offset"`
}

func (p StoredPage) Validate() error {
	if err := p.Header.Validate(); err != nil {
		return err
	}
	if len(p.Items) > 100 || p.Offset < 0 || p.Offset > p.Header.Total || int64(len(p.Items)) > p.Header.Total-p.Offset {
		return ErrUnavailable
	}
	policy, _ := Policy(p.Header.Category)
	seen := make(map[string]bool, len(p.Items))
	for i, item := range p.Items {
		score, e := strconv.ParseFloat(item.Score, 64)
		if !addressGrammar.MatchString(item.Address) || e != nil || !decimalGrammar.MatchString(item.Score) || math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > policy.MaxScore || item.Address != strings.ToLower(item.Address) || seen[item.Address] || item.Rank != p.Offset+int64(i)+1 {
			return fmt.Errorf("invalid persisted ranking page")
		}
		seen[item.Address] = true
		if i > 0 && p.Header.Category != categoryNew {
			previous, _ := strconv.ParseFloat(p.Items[i-1].Score, 64)
			if previous < score || (previous == score && p.Items[i-1].Address >= item.Address) {
				return fmt.Errorf("invalid persisted ranking order")
			}
		}
	}
	return nil
}

func (s Snapshot) Metadata() InputSnapshot {
	out := s.input
	out.Candidates = nil
	if s.input.Provenance != nil {
		p := *s.input.Provenance
		out.Provenance = &p
	}
	return out
}

func (c *Coordinator) Run(ctx context.Context, newOnly bool, report func(error)) error {
	if c == nil || report == nil {
		return ErrUnavailable
	}
	interval := 30 * time.Second
	if newOnly {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := c.Refresh(ctx, uuid.NewString(), newOnly)
		report(err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func HeavyCategoryNames() []string { return append([]string(nil), heavyCategories[:]...) }

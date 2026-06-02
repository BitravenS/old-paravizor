package events

import "time"

// Event is the base interface for all events.
type Event interface {
	EventType() string
	Timestamp() time.Time
}

// --- Pipeline lifecycle events ---

type PipelineStarted struct {
	PipelineID string
	NodeCount  int
	Time       time.Time
}

func (e PipelineStarted) EventType() string    { return "pipeline.started" }
func (e PipelineStarted) Timestamp() time.Time { return e.Time }

type PipelineCompleted struct {
	Duration    time.Duration
	TotalItems  int
	TotalErrors int
	Time        time.Time
}

func (e PipelineCompleted) EventType() string    { return "pipeline.completed" }
func (e PipelineCompleted) Timestamp() time.Time { return e.Time }

// --- Node events ---

type NodeStarted struct {
	NodeID string
	Time   time.Time
}

func (e NodeStarted) EventType() string    { return "node.started" }
func (e NodeStarted) Timestamp() time.Time { return e.Time }

type NodeCompleted struct {
	NodeID   string
	ItemsIn  int
	ItemsOut int
	Duration time.Duration
	Time     time.Time
}

func (e NodeCompleted) EventType() string    { return "node.completed" }
func (e NodeCompleted) Timestamp() time.Time { return e.Time }

type NodeError struct {
	NodeID string
	Err    error
	Fatal  bool
	Time   time.Time
}

func (e NodeError) EventType() string    { return "node.error" }
func (e NodeError) Timestamp() time.Time { return e.Time }

// --- Batch events ---

type BatchStarted struct {
	BatchID   int64
	NodeID    string
	ItemCount int
	Time      time.Time
}

func (e BatchStarted) EventType() string    { return "batch.started" }
func (e BatchStarted) Timestamp() time.Time { return e.Time }

type BatchCompleted struct {
	BatchID   int64
	NodeID    string
	ItemCount int
	Duration  time.Duration
	Time      time.Time
}

func (e BatchCompleted) EventType() string    { return "batch.completed" }
func (e BatchCompleted) Timestamp() time.Time { return e.Time }

// --- Item discovery events ---

type DomainDiscovered struct {
	DomainName string
	DomainID   int64
	Source     string
	NodeID     string
	Time       time.Time
}

func (e DomainDiscovered) EventType() string    { return "item.domain.discovered" }
func (e DomainDiscovered) Timestamp() time.Time { return e.Time }

type URLDiscovered struct {
	FullURL string
	URLID   int64
	Source  string
	NodeID  string
	Time    time.Time
}

func (e URLDiscovered) EventType() string    { return "item.url.discovered" }
func (e URLDiscovered) Timestamp() time.Time { return e.Time }

type FindingDiscovered struct {
	FindingID int64
	Title     string
	Severity  string
	Scanner   string
	NodeID    string
	Time      time.Time
}

func (e FindingDiscovered) EventType() string    { return "item.finding.discovered" }
func (e FindingDiscovered) Timestamp() time.Time { return e.Time }

// --- Scope events ---

type OutOfScopeFiltered struct {
	ItemType string
	ItemID   int64
	NodeID   string
	Reason   string
	Time     time.Time
}

func (e OutOfScopeFiltered) EventType() string    { return "scope.filtered" }
func (e OutOfScopeFiltered) Timestamp() time.Time { return e.Time }

// --- Process events ---

type ProcessStarted struct {
	ProcessID int64
	ToolName  string
	Command   string
	PID       int
	NodeID    string
	Time      time.Time
}

func (e ProcessStarted) EventType() string    { return "process.started" }
func (e ProcessStarted) Timestamp() time.Time { return e.Time }

type ProcessCompleted struct {
	ProcessID int64
	ToolName  string
	ExitCode  int
	Duration  time.Duration
	PID       int
	NodeID    string
	Time      time.Time
}

func (e ProcessCompleted) EventType() string    { return "process.completed" }
func (e ProcessCompleted) Timestamp() time.Time { return e.Time }

type ProcessOutput struct {
	ProcessID int64
	NodeID    string
	Stream    string // "stdout" | "stderr"
	Line      string
	Time      time.Time
}

func (e ProcessOutput) EventType() string    { return "process.output" }
func (e ProcessOutput) Timestamp() time.Time { return e.Time }

// --- Rate limit events ---

type RateLimitRebalanced struct {
	Allocations map[string]float64
	TotalBudget float64
	Time        time.Time
}

func (e RateLimitRebalanced) EventType() string    { return "ratelimit.rebalanced" }
func (e RateLimitRebalanced) Timestamp() time.Time { return e.Time }

type RateThrottled struct {
	NodeID  string
	OldRate float64
	NewRate float64
	Reason  string
	Time    time.Time
}

func (e RateThrottled) EventType() string    { return "ratelimit.throttled" }
func (e RateThrottled) Timestamp() time.Time { return e.Time }

// --- Checkpoint events ---

type CheckpointSaved struct {
	Time time.Time
}

func (e CheckpointSaved) EventType() string    { return "checkpoint.saved" }
func (e CheckpointSaved) Timestamp() time.Time { return e.Time }

// --- Log events ---

type LogMessage struct {
	Level   string // debug, info, warn, error
	Message string
	Fields  map[string]string
	Time    time.Time
}

func (e LogMessage) EventType() string    { return "log" }
func (e LogMessage) Timestamp() time.Time { return e.Time }

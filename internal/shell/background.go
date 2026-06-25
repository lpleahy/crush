package shell

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/pubsub"
)

const (
	// MaxBackgroundJobs is the maximum number of concurrent background jobs allowed
	MaxBackgroundJobs = 50
	// CompletedJobRetentionMinutes is how long to keep completed jobs before auto-cleanup (8 hours)
	CompletedJobRetentionMinutes = 8 * 60
)

// syncBuffer is a thread-safe wrapper around bytes.Buffer.
type syncBuffer struct {
	buf bytes.Buffer
	mu  sync.RWMutex
}

func (sb *syncBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *syncBuffer) WriteString(s string) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.WriteString(s)
}

func (sb *syncBuffer) String() string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.buf.String()
}

// BackgroundShell represents a shell running in the background.
type BackgroundShell struct {
	ID          string
	Command     string
	Description string
	Shell       *Shell
	WorkingDir  string
	ctx         context.Context
	cancel      context.CancelFunc
	stdout      *syncBuffer
	stderr      *syncBuffer
	done        chan struct{}
	exitErr     error
	completedAt atomic.Int64 // Unix timestamp when job completed (0 if still running)
	waiters     atomic.Int32 // callers currently blocked in WaitContext (agent wait-block)
}

// BackgroundShellManager manages background shell instances.
type BackgroundShellManager struct {
	shells *csync.Map[string, *BackgroundShell]
}

var (
	backgroundManager     *BackgroundShellManager
	backgroundManagerOnce sync.Once
	idCounter             atomic.Uint64
)

// newBackgroundShellManager creates a new BackgroundShellManager instance.
func newBackgroundShellManager() *BackgroundShellManager {
	return &BackgroundShellManager{
		shells: csync.NewMap[string, *BackgroundShell](),
	}
}

// GetBackgroundShellManager returns the singleton background shell manager.
func GetBackgroundShellManager() *BackgroundShellManager {
	backgroundManagerOnce.Do(func() {
		backgroundManager = newBackgroundShellManager()
	})
	return backgroundManager
}

// JobInfo is a point-in-time snapshot of a background job for display.
type JobInfo struct {
	ID          string
	Command     string
	Description string
	WorkingDir  string
	Done        bool
	Waiting     bool  // an agent tool is currently wait-blocking on this job
	ExitErr     error // set only when Done
	CompletedAt int64 // Unix seconds when the job finished (0 while running)
}

// JobEvent is published whenever a background job's lifecycle or wait state
// changes (started, blocked/unblocked, completed, killed).
type JobEvent struct {
	Job JobInfo
}

// jobBroker fans background-job state changes out to subscribers (e.g. the TUI
// sidebar). It mirrors the per-resource brokers used for MCP/LSP/skills.
var jobBroker = pubsub.NewBroker[JobEvent]()

// SubscribeJobEvents returns a channel of background-job lifecycle events.
func SubscribeJobEvents(ctx context.Context) <-chan pubsub.Event[JobEvent] {
	return jobBroker.Subscribe(ctx)
}

// ShutdownJobBroker stops the job event broker (call on app shutdown).
func ShutdownJobBroker() {
	jobBroker.Shutdown()
}

// info snapshots the shell's current state. exitErr is read only after the done
// channel is observed closed, so the read is race-free.
func (bs *BackgroundShell) info() JobInfo {
	ji := JobInfo{
		ID:          bs.ID,
		Command:     bs.Command,
		Description: bs.Description,
		WorkingDir:  bs.WorkingDir,
		CompletedAt: bs.completedAt.Load(),
	}
	select {
	case <-bs.done:
		ji.Done = true
		ji.ExitErr = bs.exitErr
	default:
	}
	ji.Waiting = !ji.Done && bs.waiters.Load() > 0
	return ji
}

func publishJob(t pubsub.EventType, bs *BackgroundShell) {
	jobBroker.Publish(t, JobEvent{Job: bs.info()})
}

// JobsSnapshot returns a snapshot of every tracked background job.
func (m *BackgroundShellManager) JobsSnapshot() []JobInfo {
	out := make([]JobInfo, 0, m.shells.Len())
	for bs := range m.shells.Seq() {
		out = append(out, bs.info())
	}
	return out
}

// Start creates and starts a new background shell with the given command.
func (m *BackgroundShellManager) Start(ctx context.Context, workingDir string, blockFuncs []BlockFunc, command string, description string) (*BackgroundShell, error) {
	// Check job limit
	if m.shells.Len() >= MaxBackgroundJobs {
		return nil, fmt.Errorf("maximum number of background jobs (%d) reached. Please terminate or wait for some jobs to complete", MaxBackgroundJobs)
	}

	id := fmt.Sprintf("%03X", idCounter.Add(1))

	shell := NewShell(&Options{
		WorkingDir: workingDir,
		BlockFuncs: blockFuncs,
	})

	shellCtx, cancel := context.WithCancel(ctx)

	bgShell := &BackgroundShell{
		ID:          id,
		Command:     command,
		Description: description,
		WorkingDir:  workingDir,
		Shell:       shell,
		ctx:         shellCtx,
		cancel:      cancel,
		stdout:      &syncBuffer{},
		stderr:      &syncBuffer{},
		done:        make(chan struct{}),
	}

	m.shells.Set(id, bgShell)
	publishJob(pubsub.CreatedEvent, bgShell)

	go func() {
		defer func() {
			close(bgShell.done)
			publishJob(pubsub.UpdatedEvent, bgShell)
		}()

		err := shell.ExecStream(shellCtx, command, bgShell.stdout, bgShell.stderr)

		bgShell.exitErr = err
		bgShell.completedAt.Store(time.Now().Unix())
	}()

	return bgShell, nil
}

// Get retrieves a background shell by ID.
func (m *BackgroundShellManager) Get(id string) (*BackgroundShell, bool) {
	return m.shells.Get(id)
}

// Remove removes a background shell from the manager without terminating it.
// This is useful when a shell has already completed and you just want to clean up tracking.
func (m *BackgroundShellManager) Remove(id string) error {
	bs, ok := m.shells.Take(id)
	if !ok {
		return fmt.Errorf("background shell not found: %s", id)
	}
	publishJob(pubsub.DeletedEvent, bs)
	return nil
}

// Kill terminates a background shell by ID.
func (m *BackgroundShellManager) Kill(id string) error {
	shell, ok := m.shells.Take(id)
	if !ok {
		return fmt.Errorf("background shell not found: %s", id)
	}

	shell.cancel()
	<-shell.done
	publishJob(pubsub.DeletedEvent, shell)
	return nil
}

// BackgroundShellInfo contains information about a background shell.
type BackgroundShellInfo struct {
	ID          string
	Command     string
	Description string
}

// List returns all background shell IDs.
func (m *BackgroundShellManager) List() []string {
	ids := make([]string, 0, m.shells.Len())
	for id := range m.shells.Seq2() {
		ids = append(ids, id)
	}
	return ids
}

// Cleanup removes completed jobs that have been finished for more than the retention period
func (m *BackgroundShellManager) Cleanup() int {
	now := time.Now().Unix()
	retentionSeconds := int64(CompletedJobRetentionMinutes * 60)

	var toRemove []string
	for shell := range m.shells.Seq() {
		completedAt := shell.completedAt.Load()
		if completedAt > 0 && now-completedAt > retentionSeconds {
			toRemove = append(toRemove, shell.ID)
		}
	}

	for _, id := range toRemove {
		m.Remove(id)
	}

	return len(toRemove)
}

// KillAll terminates all background shells. The provided context bounds how
// long the function waits for each shell to exit.
func (m *BackgroundShellManager) KillAll(ctx context.Context) {
	shells := slices.Collect(m.shells.Seq())
	m.shells.Reset(map[string]*BackgroundShell{})

	var wg sync.WaitGroup
	for _, shell := range shells {
		wg.Go(func() {
			shell.cancel()
			select {
			case <-shell.done:
			case <-ctx.Done():
			}
		})
	}
	wg.Wait()
}

// GetOutput returns the current output of a background shell.
func (bs *BackgroundShell) GetOutput() (stdout string, stderr string, done bool, err error) {
	select {
	case <-bs.done:
		return bs.stdout.String(), bs.stderr.String(), true, bs.exitErr
	default:
		return bs.stdout.String(), bs.stderr.String(), false, nil
	}
}

// IsDone checks if the background shell has finished execution.
func (bs *BackgroundShell) IsDone() bool {
	select {
	case <-bs.done:
		return true
	default:
		return false
	}
}

// Wait blocks until the background shell completes.
func (bs *BackgroundShell) Wait() {
	<-bs.done
}

func (bs *BackgroundShell) WaitContext(ctx context.Context) bool {
	// Mark the job as being waited on so the UI can show it as blocked.
	bs.waiters.Add(1)
	publishJob(pubsub.UpdatedEvent, bs)
	defer func() {
		bs.waiters.Add(-1)
		publishJob(pubsub.UpdatedEvent, bs)
	}()
	select {
	case <-bs.done:
		return true
	case <-ctx.Done():
		return false
	}
}

package embeddings

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// How many inference runs each pipeline serves before it is closed and rebuilt
// from scratch.
//
// The pure-Go backend never gives a buffer back, so a pipeline only ever grows
// and the sole way to reclaim it is to throw the pipeline away. What one run
// costs differs sharply between the two models, so the intervals do too:
//
//   - The classifier is pinned to one tensor shape by
//     [pipelines.WithFixedPadding], so it compiles a single graph and a run adds
//     only its own scratch. It can serve a long interval, and wants to: the
//     125M-parameter model takes ~9s to reload.
//
//   - Feature extraction has no such pin. hugot exposes WithFixedPadding only
//     for *TextClassificationPipeline, and on the Go backend sequence padding is
//     off (backends.backend.go passes runtime == BackendXLA for it), so
//     WithGoMLXSequenceBuckets is inert too. Every distinct token length
//     therefore compiles and retains its own graph, each holding its own copy of
//     the weights as constants plus a prepacked copy inside matmul's sync.Once:
//     **measured 153 MiB per distinct sequence length**, against 348 MiB for the
//     first. Fifty distinct lengths is ~8 GiB, which is exactly where a
//     production pod ended up.
//
// So the embedding interval is a memory budget, not a throughput knob: at worst
// one new graph per run, 20 runs caps the pipeline near 3 GiB. The model is
// small enough (~87 MB) that reloading it that often is affordable.
const (
	embedRecycleInterval = 20
	moodRecycleInterval  = 1000
)

// maxPipelineAge rebuilds a pipeline that has been alive this long whatever its
// run count.
//
// Run counts alone are no bound on a bot: live traffic measured ~1 message every
// 4 minutes, so 1000 runs is nearly three days away, while the set of distinct
// sequence lengths saturates within the first few hundred messages. The process
// parked on 9 GiB of graphs for days, 1.4 GiB under its limit, having stopped
// doing anything that needed them.
//
// This is checked on the same deferred path as the run count, so it releases on
// the first use after expiry rather than the moment it expires. A process that
// goes completely silent keeps what it has until something asks it to work
// again — bounding that too would mean a timer goroutine for no practical gain.
const maxPipelineAge = 15 * time.Minute

// pipelineGuard gates one pipeline's lazy construction and owns the decision to
// tear it down and rebuild.
//
// A sync.Once cannot be reset, and resetting is the point; see the intervals
// above.
//
// attempted rather than a success flag, so a failed construction is remembered:
// a backfill calling this a million times must not retry a failing model
// download on every message. A recycle clears it.
type pipelineGuard struct {
	// name, interval and runLock are set at construction and never change.
	// runLock is the lock that serializes this pipeline's execution, held during
	// a recycle so one cannot land mid-inference and free buffers in use.
	name     string
	interval int
	runLock  *sync.Mutex

	mu        sync.Mutex
	attempted bool
	err       error
	builtAt   time.Time

	// runs counts inferences served since the pipeline was built. Atomic because
	// it is incremented under runLock but read without it.
	runs atomic.Int64
}

// ensure runs build once, returning whatever it produced on every later call.
func (g *pipelineGuard) ensure(build func() error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.attempted {
		g.err = build()
		g.attempted = true
		g.builtAt = time.Now()
	}
	return g.err
}

// ran counts one inference. Called before the run rather than after, because a
// run that fails has allocated its buffers all the same.
func (g *pipelineGuard) ran() {
	g.runs.Add(1)
}

// due reports whether the pipeline is worth rebuilding: it has served its
// interval, or it has outlived maxPipelineAge. Either way at least one run must
// have happened, so a pipeline built and not yet used is left alone.
func (g *pipelineGuard) due() bool {
	if g.runs.Load() == 0 {
		return false
	}
	if g.runs.Load() >= int64(g.interval) {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.builtAt.IsZero() && time.Since(g.builtAt) >= maxPipelineAge
}

// forget drops the memo and the run count so the next ensure rebuilds.
func (g *pipelineGuard) forget() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.attempted, g.err = false, nil
	g.builtAt = time.Time{}
	g.runs.Store(0)
}

// recycle closes the pipeline and forgets it, so the next use rebuilds it from
// scratch.
func (g *pipelineGuard) recycle() error {
	g.runLock.Lock()
	defer g.runLock.Unlock()

	if session == nil {
		return nil
	}
	// Forget before closing: ClosePipeline unregisters the name before it tears
	// the model down, so even a close that errors leaves the name free for the
	// next build. Forgetting first means a close error cannot strand the guard on
	// a pipeline that is already gone.
	g.forget()

	slog.Debug("recycling inference pipeline", slog.String("pipeline", g.name))
	return session.ClosePipeline(g.name)
}

// recycleIfDue rebuilds the pipeline once it is due.
//
// Deferred by the exported entry points, so it runs with no lock held and after
// the caller's own work is done: whoever crosses the threshold pays nothing, and
// the next caller pays the reload. A failure is logged rather than returned —
// the inference it follows already succeeded, and the only consequence is that
// the memory this call meant to release is kept a while longer.
func (g *pipelineGuard) recycleIfDue() {
	if !g.due() {
		return
	}
	if err := g.recycle(); err != nil {
		slog.Warn("failed to recycle inference pipeline",
			slog.String("pipeline", g.name), slog.Any("err", err))
	}
}

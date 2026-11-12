package embeddings

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// recycleInterval is how many inference runs a pipeline serves before it is
// closed and rebuilt from scratch.
//
// This exists because of two faults in the pure-Go inference backend, both of
// which were fixed upstream in the dependency bumps of late 2026 (hugot v0.8.5,
// gomlx/compute v0.1.16) and are kept here only as a brake:
//
//   - Buffers taken per execution and never returned: measured at 317 KiB per
//     message, 1.96 GiB/hour, which killed a 12 GiB pod about three hours into a
//     backfill. Now measured at 0.1 KiB per run over 200 runs, flat.
//
//   - One compiled graph retained per distinct sequence length, each holding its
//     own copy of the weights: 153 MiB per novel length, ~8 GiB once a process
//     had seen fifty of them. hugot now builds a dynamic-axis graph for text
//     models, so the embedding pipeline serves every length from one graph:
//     measured at 0.2 MiB per novel length, 40 of them costing 7 MiB in total.
//
// With both fixed there is no longer any reason for the two pipelines to differ,
// so one interval covers both. It is deliberately loose: at these rates it
// reclaims almost nothing, and its real job is to be a bound that exists at all
// if either fault returns in a future bump.
//
// Note the classifier does not get the dynamic-axis path. Its ONNX export folds
// the position-embedding add to a constant [1, 513, 768], which cannot broadcast
// against a dynamic sequence axis, so hugot prints "dynamic axes failed for
// model" once per load and falls back to concrete shapes. That is why the
// classifier still needs [pipelines.WithFixedPadding] to hold one shape.
const recycleInterval = 1000

// maxPipelineAge rebuilds a pipeline that has been alive this long whatever its
// run count.
//
// Run counts alone are no bound on a bot: live traffic measured ~1 message every
// 4 minutes, so 1000 runs is nearly three days away. This is now the trigger
// that does the real work, and what it reclaims is the models themselves —
// ~2.2 GiB for the classifier and ~350 MiB for the embedder, held by a process
// that has stopped doing anything that needs them.
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
	// name and runLock are set at construction and never change. runLock is the
	// lock that serializes this pipeline's execution, held during a recycle so
	// one cannot land mid-inference and free buffers in use.
	name    string
	runLock *sync.Mutex

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

// due reports whether the pipeline is worth rebuilding: it has served
// recycleInterval runs, or it has outlived maxPipelineAge. Either way at least one run must
// have happened, so a pipeline built and not yet used is left alone.
func (g *pipelineGuard) due() bool {
	if g.runs.Load() == 0 {
		return false
	}
	if g.runs.Load() >= recycleInterval {
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

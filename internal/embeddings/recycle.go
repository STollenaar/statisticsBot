package embeddings

import (
	"log/slog"
	"sync"
	"sync/atomic"
)

// RecycleInterval is how many inference runs a pipeline serves before it is
// closed and rebuilt from scratch.
//
// The pure-Go backend retains a few hundred KiB of buffers per run and never
// returns them (measured: 317 KiB per message, 1.96 GiB/hour of backfill), so
// without this every long-lived process grows until it is killed — a backfill
// in hours, a quiet bot over weeks of ordinary message traffic. Closing the
// pipeline releases all of it, measured back to baseline, at the cost of a ~9s
// reload on the next call.
//
// 1000 runs puts that reload at well under 1% of the work it covers, and caps
// the leak at ~310 MiB on top of the model's own ~2.4 GiB.
const RecycleInterval = 1000

// pipelineGuard gates a pipeline's lazy construction and allows it to be torn
// down and rebuilt.
//
// A sync.Once cannot be reset, and resetting is the point; see RecycleInterval.
//
// attempted rather than a success flag, so a failed construction is remembered:
// a backfill calling this a million times must not retry a failing model
// download on every message. A recycle clears it.
type pipelineGuard struct {
	mu        sync.Mutex
	attempted bool
	err       error

	// runs counts inferences served since the pipeline was built. Atomic because
	// it is incremented under the pipeline's run lock but read without it.
	runs atomic.Int64
}

// ensure runs build once, returning whatever it produced on every later call.
func (g *pipelineGuard) ensure(build func() error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.attempted {
		g.err = build()
		g.attempted = true
	}
	return g.err
}

// ran counts one inference. Called before the run rather than after, because a
// run that fails has allocated its buffers all the same.
func (g *pipelineGuard) ran() {
	g.runs.Add(1)
}

// due reports whether the pipeline has served enough runs to be worth rebuilding.
func (g *pipelineGuard) due() bool {
	return g.runs.Load() >= RecycleInterval
}

// forget drops the memo and the run count so the next ensure rebuilds.
func (g *pipelineGuard) forget() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.attempted, g.err = false, nil
	g.runs.Store(0)
}

// recycle closes a pipeline and forgets it, so the next use rebuilds it from
// scratch. runLock is the lock that serializes that pipeline's execution, held
// here so a recycle cannot land mid-inference and free buffers in use.
func recycle(name string, guard *pipelineGuard, runLock *sync.Mutex) error {
	runLock.Lock()
	defer runLock.Unlock()

	if session == nil {
		return nil
	}
	// Forget before closing: ClosePipeline unregisters the name before it tears
	// the model down, so even a close that errors leaves the name free for the
	// next build. Forgetting first means a close error cannot strand the guard on
	// a pipeline that is already gone.
	guard.forget()

	slog.Debug("recycling inference pipeline", slog.String("pipeline", name))
	return session.ClosePipeline(name)
}

// recycleIfDue rebuilds the pipeline once it has served RecycleInterval runs.
//
// Deferred by the exported entry points, so it runs with no lock held and after
// the caller's own work is done: whoever crosses the threshold pays nothing, and
// the next caller pays the reload. A failure is logged rather than returned —
// the inference it follows already succeeded, and the only consequence is that
// the memory this call meant to release is kept a while longer.
func recycleIfDue(name string, guard *pipelineGuard, runLock *sync.Mutex) {
	if !guard.due() {
		return
	}
	if err := recycle(name, guard, runLock); err != nil {
		slog.Warn("failed to recycle inference pipeline",
			slog.String("pipeline", name), slog.Any("err", err))
	}
}

func recycleMoodIfDue()       { recycleIfDue(moodPipelineName, &moodGuard, &moodMu) }
func recycleEmbeddingsIfDue() { recycleIfDue(embedPipelineName, &embedGuard, &runMu) }

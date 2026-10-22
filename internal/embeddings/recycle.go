package embeddings

import (
	"log/slog"
	"sync"
)

// pipelineGuard gates a pipeline's lazy construction and allows it to be torn
// down and rebuilt.
//
// A sync.Once cannot be reset, and resetting is the point: gomlx's Go backend
// retains buffers per execution and never returns them, so a long backfill
// grows without bound. Closing the pipeline releases the lot — measured back to
// baseline — and the next call rebuilds it.
//
// attempted rather than a success flag, so a failed construction is remembered:
// a backfill calling this a million times must not retry a failing model
// download on every message. An explicit Recycle clears it.
type pipelineGuard struct {
	mu        sync.Mutex
	attempted bool
	err       error
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

// forget drops the memo so the next ensure rebuilds.
func (g *pipelineGuard) forget() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.attempted, g.err = false, nil
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

// RecycleMood closes the mood classifier, releasing the buffers the Go backend
// accumulated, and leaves the next ScoreMood to rebuild it. Reloading costs a
// few seconds; see the backfill that calls this for how often it is worth it.
func RecycleMood() error {
	return recycle(moodPipelineName, &moodGuard, &moodMu)
}

// RecycleEmbeddings is RecycleMood for the feature-extraction pipeline, which
// leaks the same way and additionally compiles a graph per distinct input
// length, so its growth tracks the variety of messages it has seen.
func RecycleEmbeddings() error {
	return recycle(embedPipelineName, &embedGuard, &runMu)
}

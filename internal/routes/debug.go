package routes

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"runtime"
	"runtime/debug"

	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/embeddings"
	"github.com/stollenaar/statisticsbot/internal/jobs"
	"github.com/stollenaar/statisticsbot/internal/util"
)

// addDebug registers the debug endpoint only when DEBUG is set.
//
// Not registered rather than registered-and-refusing: with no handler the route
// 404s like any unknown path, so nothing advertises that the endpoint exists.
// It still sits behind the shared-token gate like every other route, because it
// reports memory figures, model names and row counts.
func addDebug(mux *http.ServeMux) {
	if !util.ConfigFile.DEBUG {
		return
	}
	slog.Info("DEBUG is set, registering GET /debug")
	mux.HandleFunc("GET /debug", debugSnapshot)
}

type debugResponse struct {
	Build     buildInfo         `json:"build"`
	Runtime   runtimeInfo       `json:"runtime"`
	Container containerInfo     `json:"container"`
	DuckDB    any               `json:"duckdb"`
	Models    modelInfo         `json:"models"`
	Jobs      map[string]any    `json:"jobs"`
	Errors    map[string]string `json:"errors,omitempty"`
}

type buildInfo struct {
	GoVersion string `json:"goVersion"`
	Revision  string `json:"revision,omitempty"`
	Modified  string `json:"modified,omitempty"`
}

type runtimeInfo struct {
	NumCPU         int    `json:"numCPU"`
	GOMAXPROCS     int    `json:"gomaxprocs"`
	Goroutines     int    `json:"goroutines"`
	HeapAllocBytes uint64 `json:"heapAllocBytes"`
	HeapSysBytes   uint64 `json:"heapSysBytes"`
	TotalSysBytes  uint64 `json:"totalSysBytes"`
	NumGC          uint32 `json:"numGC"`
}

type containerInfo struct {
	// Bytes for machines, strings for eyes. The whole reason to read this
	// endpoint is usually to compare the two numbers, and doing that in bytes is
	// needless arithmetic under pressure.
	MemoryLimitBytes  int64   `json:"memoryLimitBytes,omitempty"`
	MemoryLimit       string  `json:"memoryLimit,omitempty"`
	MemoryUsedBytes   int64   `json:"memoryUsedBytes,omitempty"`
	MemoryUsed        string  `json:"memoryUsed,omitempty"`
	MemoryUsedPercent float64 `json:"memoryUsedPercent,omitempty"`
	Note              string  `json:"note,omitempty"`
}

type modelInfo struct {
	Embedding string `json:"embedding"`
	Mood      string `json:"mood"`
}

// debugSnapshot reports what the process looks like right now.
//
// Everything here is either in-memory or a counter the source already tracks:
// no table scans, no inference, nothing that costs more to ask than to answer.
// That matters because the reason to reach for this endpoint is usually that the
// process is already under pressure.
func debugSnapshot(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	resp := debugResponse{
		Runtime: runtimeInfo{
			NumCPU:         runtime.NumCPU(),
			GOMAXPROCS:     runtime.GOMAXPROCS(0),
			Goroutines:     runtime.NumGoroutine(),
			HeapAllocBytes: mem.HeapAlloc,
			HeapSysBytes:   mem.HeapSys,
			TotalSysBytes:  mem.Sys,
			NumGC:          mem.NumGC,
		},
		Models: modelInfo{
			Embedding: embeddings.ModelName(),
			Mood:      embeddings.MoodModelName(),
		},
		Jobs: map[string]any{
			jobs.KeyEmbeddings: jobs.EmbeddingBackfill(),
			jobs.KeyMoods:      jobs.MoodBackfill(),
			jobs.KeyMessages:   jobs.MessageSync(),
		},
		Errors: map[string]string{},
	}

	resp.Build.GoVersion = runtime.Version()
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range bi.Settings {
			switch setting.Key {
			case "vcs.revision":
				resp.Build.Revision = setting.Value
			case "vcs.modified":
				resp.Build.Modified = setting.Value
			}
		}
	}

	// Absent outside a container, or under cgroup v1. Saying so beats reporting
	// a zero that reads like "no memory".
	limit, haveLimit := util.CgroupMemoryLimit()
	if haveLimit {
		resp.Container.MemoryLimitBytes = limit
		resp.Container.MemoryLimit = humanBytes(limit)
	} else {
		resp.Container.Note = "no cgroup v2 memory limit visible"
	}
	if used, ok := util.CgroupMemoryUsage(); ok {
		resp.Container.MemoryUsedBytes = used
		resp.Container.MemoryUsed = humanBytes(used)
		if haveLimit && limit > 0 {
			resp.Container.MemoryUsedPercent = math.Round(float64(used)/float64(limit)*1000) / 10
		}
	}

	// A failure here must not fail the whole snapshot: the runtime and job
	// figures above are often the ones being chased.
	if info, err := database.GetDebugInfo(); err != nil {
		resp.Errors["duckdb"] = err.Error()
	} else {
		resp.DuckDB = info
	}

	if len(resp.Errors) == 0 {
		resp.Errors = nil
	}
	writeJSON(w, http.StatusOK, resp)
}

// humanBytes renders a byte count in binary units, which is what both cgroup
// limits and Kubernetes resource limits are expressed in.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.2f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.2f PiB", value/unit)
}

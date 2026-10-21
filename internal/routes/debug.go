package routes

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

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
	slog.Info("DEBUG is set, registering GET /debug and /debug/pprof/")
	mux.HandleFunc("GET /debug", debugSnapshot)

	// The snapshot above says how much memory is held; pprof says by what.
	//
	//	go tool pprof http://host:8080/debug/pprof/heap
	//
	// Reached through the same token gate as everything else, which matters
	// rather more here: these handlers expose symbol names, command line and
	// stacks. /debug/pprof/profile and /trace block for their sample window, so
	// they are deliberately slow rather than broken.
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
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
	NumCPU     int `json:"numCPU"`
	GOMAXPROCS int `json:"gomaxprocs"`
	Goroutines int `json:"goroutines"`

	// Where the memory actually is. HeapAlloc alone cannot tell "3 GiB of live
	// data" from "3 GiB the runtime is holding but not using":
	//   HeapInuse - HeapAlloc  is fragmentation inside used spans
	//   HeapIdle  - HeapReleased  is heap held back from the OS, reusable and
	//                             still counted against the container limit
	//   Sys - HeapSys  is everything non-heap: stacks, GC metadata, the runtime
	HeapAlloc     string `json:"heapAlloc"`
	HeapInuse     string `json:"heapInuse"`
	HeapIdle      string `json:"heapIdle"`
	HeapReleased  string `json:"heapReleased"`
	HeapSys       string `json:"heapSys"`
	StackInuse    string `json:"stackInuse"`
	GCSys         string `json:"gcSys"`
	OtherSys      string `json:"otherSys"`
	TotalSys      string `json:"totalSys"`
	NextGCTarget  string `json:"nextGCTarget"`
	GOMEMLIMIT    string `json:"gomemlimit"`
	ProcessRSS    string `json:"processRSS,omitempty"`
	NumGC         uint32 `json:"numGC"`
	GCCPUFraction string `json:"gcCPUFraction"`

	// Bytes for machines; the strings above are the same values formatted.
	HeapAllocBytes uint64 `json:"heapAllocBytes"`
	HeapSysBytes   uint64 `json:"heapSysBytes"`
	TotalSysBytes  uint64 `json:"totalSysBytes"`
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
			HeapAlloc:      humanBytes(int64(mem.HeapAlloc)),
			HeapInuse:      humanBytes(int64(mem.HeapInuse)),
			HeapIdle:       humanBytes(int64(mem.HeapIdle)),
			HeapReleased:   humanBytes(int64(mem.HeapReleased)),
			HeapSys:        humanBytes(int64(mem.HeapSys)),
			StackInuse:     humanBytes(int64(mem.StackInuse)),
			GCSys:          humanBytes(int64(mem.GCSys)),
			OtherSys:       humanBytes(int64(mem.OtherSys)),
			TotalSys:       humanBytes(int64(mem.Sys)),
			NextGCTarget:   humanBytes(int64(mem.NextGC)),
			GOMEMLIMIT:     memoryLimitLabel(),
			ProcessRSS:     processRSS(),
			NumGC:          mem.NumGC,
			GCCPUFraction:  fmt.Sprintf("%.3f%%", mem.GCCPUFraction*100),
			HeapAllocBytes: mem.HeapAlloc,
			HeapSysBytes:   mem.HeapSys,
			TotalSysBytes:  mem.Sys,
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

// memoryLimitLabel reports the Go runtime's soft memory limit. Unset it is
// math.MaxInt64, which means the runtime will grow the heap until the kernel
// objects — setting GOMEMLIMIT below the container limit makes the collector
// work harder instead, which only helps if the memory is actually reclaimable.
func memoryLimitLabel() string {
	limit := debug.SetMemoryLimit(-1) // -1 reads without changing
	if limit == math.MaxInt64 {
		return "unset (no soft limit)"
	}
	return humanBytes(limit)
}

// processRSS reads the process's resident set size, which is what the container
// limit is actually enforced against — Go's own accounting excludes the models
// and anything else allocated outside the heap.
func processRSS() string {
	raw, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return ""
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return ""
		}
		return humanBytes(kb * 1024)
	}
	return ""
}

// Package embeddings produces sentence embeddings locally using a hugot
// feature-extraction pipeline (default: sentence-transformers/all-MiniLM-L6-v2)
// running on hugot's pure-Go backend, so no ONNX runtime system library is
// required. The model is downloaded from HuggingFace on first use.
package embeddings

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

const (
	MODEL_NAME = "sentence-transformers/all-MiniLM-L6-v2"

	// all-MiniLM-L6-v2 has a hard 512-token context window (position embeddings),
	// and sentence-transformers truncates to 256 by default. The Go-backend
	// tokenizer does not reliably truncate, so we cap input length ourselves.
	// firstAttemptRunes stays well under 512 tokens for typical text; on the rare
	// token-dense input that still overflows we shrink and retry.
	firstAttemptRunes = 1200
	minAttemptRunes   = 128
)

var (
	initOnce sync.Once
	initErr  error

	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline

	runMu sync.Mutex
)

// ModelName returns the configured embedding model identifier. Stored alongside
// each vector so search only compares vectors produced by the same model.
func ModelName() string {
	return MODEL_NAME
}

func initPipeline() {
	ctx := context.Background()

	session, initErr = hugot.NewGoSession(ctx)
	if initErr != nil {
		return
	}

	modelPath, err := ensureModel(ctx, "sentence-transformers/all-MiniLM-L6-v2")

	if err != nil {
		initErr = err
		return
	}

	config := hugot.FeatureExtractionConfig{
		ModelPath: modelPath,
		Name:      "semantic-embeddings",
	}

	pipeline, initErr = hugot.NewPipeline(session, config)
}

// ensureModel returns the local path to the model, downloading it from
// HuggingFace into MODELS_PATH the first time.
func ensureModel(ctx context.Context, name string) (string, error) {

	opts := hugot.NewDownloadOptions()
	opts.OnnxFilePath = "onnx/model.onnx"

	modelsDir := "./models/"
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create models directory: %w", err)
	}

	return hugot.DownloadModel(ctx, name, modelsDir, opts)
}

// Embed returns the embedding vector for a single input string. The pipeline is
// initialized (and the model downloaded) on first call.
//
// Inputs longer than the model's 512-token context window would crash the
// pipeline during graph construction, so the text is capped up front and, if it
// still overflows, halved and retried until it fits.
func Embed(text string) ([]float32, error) {
	initOnce.Do(initPipeline)
	if initErr != nil {
		return nil, initErr
	}

	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil, errors.New("cannot embed empty text")
	}

	limit := len(runes)
	if limit > firstAttemptRunes {
		limit = firstAttemptRunes
	}

	var lastErr error
	for {
		vec, err := runEmbed(string(runes[:limit]))
		if err == nil {
			return vec, nil
		}
		lastErr = err
		if limit <= minAttemptRunes {
			return nil, lastErr
		}
		if limit /= 2; limit < minAttemptRunes {
			limit = minAttemptRunes
		}
	}
}

// runEmbed runs a single input through the pipeline under the run lock.
func runEmbed(text string) ([]float32, error) {
	runMu.Lock()
	defer runMu.Unlock()

	out, err := pipeline.RunPipeline(context.Background(), []string{text})
	if err != nil {
		return nil, err
	}
	if len(out.Embeddings) == 0 {
		return nil, errors.New("no embedding returned")
	}
	return out.Embeddings[0], nil
}

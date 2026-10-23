package embeddings

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

const (
	// MOOD_MODEL_NAME classifies text into go_emotions' 28 labels. It is picked
	// over j-hartmann/emotion-english-distilroberta-base because that repo ships
	// only PyTorch and TensorFlow weights, and hugot loads ONNX exclusively.
	MOOD_MODEL_NAME = "SamLowe/roberta-base-go_emotions-onnx"

	// moodOnnxPath is where this repo keeps its export. The quantized variant
	// beside it would be faster and less accurate.
	moodOnnxPath = "onnx/model.onnx"

	// moodSequenceLength pads every input to a fixed token length, which is what
	// keeps this pipeline's memory bounded.
	//
	// On the Go backend hugot disables sequence padding entirely
	// (createInputTensorsGoMLX is called with padSequenceDimension = runtime ==
	// XLA), and the Go backend's default JIT cache is unlimited. Every distinct
	// token length therefore compiled and retained its own graph for a 125M
	// parameter model: scoring 68 messages of assorted lengths peaked at 24 GiB.
	// Forcing one shape means one compiled graph, which measured ~2-3 GiB for the
	// same work.
	//
	// Bucket options are not an alternative: WithGoMLXSequenceBuckets has no
	// effect while sequence padding is off, and bounding the cache without
	// padding just makes most inputs fail to compile.
	//
	// 64 tokens is roughly 250 characters, against a median message of 33 and a
	// mean of 56, so little is truncated. Lowering it is faster and raising it
	// truncates less: measured 1.7/s at 32, 0.9/s at 64, 0.4/s at 128.
	moodSequenceLength = 64

	// moodMaxRunes caps input before tokenizing. Past roughly four characters per
	// token there is nothing left for the fixed padding above to keep, so the
	// extra tokenizer work would be wasted.
	moodMaxRunes = moodSequenceLength * 4
)

var (
	moodGuard pipelineGuard

	// moodMu serializes scoring for the same reason runMu does for embedding:
	// one shared pipeline, and the gain from parallel calls is not worth the
	// risk until it is measured.
	moodMu sync.Mutex
)

// MoodModelName returns the configured classifier identifier. Stored alongside
// each score so a model change — which alters both the label set and its order —
// cannot be mistaken for comparable data.
func MoodModelName() string {
	return MOOD_MODEL_NAME
}

// Mood is one message's emotion distribution. Labels and Scores are parallel and
// in the model's own label order, so Scores alone is a stable vector to store.
type Mood struct {
	Label  string
	Score  float32
	Labels []string
	Scores []float32
}

// moodClassifier returns the classification pipeline, creating it on the shared
// session the first time. Like embedPipeline it reads the pipeline back from the
// session by name instead of caching it.
func moodClassifier() (*pipelines.TextClassificationPipeline, error) {
	if err := moodGuard.ensure(func() error {
		s, err := ensureSession()
		if err != nil {
			return err
		}

		modelPath, err := ensureModelWithOnnx(context.Background(), MOOD_MODEL_NAME, moodOnnxPath)
		if err != nil {
			return err
		}

		_, err = s.NewPipeline(hugot.TextClassificationConfig{
			ModelPath: modelPath,
			// Named explicitly: hugot refuses to guess when a model directory
			// holds more than one .onnx, which this one does as soon as the
			// quantized variant mentioned above is fetched beside the full one.
			OnnxFilename: path.Base(moodOnnxPath),
			Name:         moodPipelineName,
			Options: []hugot.TextClassificationOption{
				// go_emotions is multi-label: a message can be both "joy" and
				// "gratitude", so the scores are independent sigmoids rather than
				// a softmax that sums to one.
				pipelines.WithMultiLabel(),
				pipelines.WithSigmoid(),
				// See moodSequenceLength: this is the memory bound, not a tuning
				// knob.
				pipelines.WithFixedPadding(moodSequenceLength),
			},
		})
		return err
	}); err != nil {
		return nil, err
	}
	return session.GetPipeline[*pipelines.TextClassificationPipeline](moodPipelineName)
}

// ScoreMood returns the emotion distribution for a single input string. The
// pipeline is initialized (and the model downloaded) on first call.
//
// Unlike Embed there is no shrink-and-retry loop: fixed padding gives every
// input the same tensor shape and silently truncates anything longer, so an
// over-long input cannot fail to compile.
func ScoreMood(text string) (Mood, error) {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return Mood{}, errors.New("cannot score empty text")
	}

	// Every caller counts toward the recycle interval, not just the backfill:
	// the listener scores each posted and edited message, and that path runs for
	// as long as the process does.
	defer recycleMoodIfDue()
	return runScore(string(runes[:min(len(runes), moodMaxRunes)]))
}

// runScore runs a single input through the classifier under the mood lock.
//
// The classifier is fetched here rather than passed in, so it is only ever
// obtained and used while the lock is held. That is what makes recycling safe:
// a concurrent recycle takes the same lock, so it cannot close the model out
// from under a run, and a caller cannot be left holding a pipeline that has
// since been torn down.
func runScore(text string) (Mood, error) {
	moodMu.Lock()
	defer moodMu.Unlock()

	classifier, err := moodClassifier()
	if err != nil {
		return Mood{}, err
	}
	moodGuard.ran()

	out, err := classifier.RunPipeline(context.Background(), []string{text})
	if err != nil {
		return Mood{}, err
	}
	if len(out.ClassificationOutputs) == 0 || len(out.ClassificationOutputs[0]) == 0 {
		return Mood{}, errors.New("no classification returned")
	}

	classes := out.ClassificationOutputs[0]
	mood := Mood{
		Labels: make([]string, len(classes)),
		Scores: make([]float32, len(classes)),
	}
	for i, c := range classes {
		mood.Labels[i] = c.Label
		mood.Scores[i] = c.Score
		// Highest-scoring label, kept denormalized so the common "what was the
		// mood" query does not have to unpack the whole vector.
		if c.Score > mood.Score {
			mood.Label, mood.Score = c.Label, c.Score
		}
	}
	if mood.Label == "" {
		return Mood{}, fmt.Errorf("classifier returned %d classes but no scores", len(classes))
	}
	return mood, nil
}

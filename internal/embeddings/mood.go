package embeddings

import (
	"context"
	"errors"
	"fmt"
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
	moodOnce sync.Once
	moodErr  error

	moodPipeline *pipelines.TextClassificationPipeline

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

func initMoodPipeline() {
	ctx := context.Background()

	// Reuse the session the embedding pipeline set up, so both models share one
	// hugot runtime rather than each holding their own.
	initOnce.Do(initPipeline)
	if initErr != nil {
		moodErr = initErr
		return
	}

	modelPath, err := ensureModelWithOnnx(ctx, MOOD_MODEL_NAME, moodOnnxPath)
	if err != nil {
		moodErr = err
		return
	}

	config := hugot.TextClassificationConfig{
		ModelPath: modelPath,
		Name:      "mood-classification",
		Options: []hugot.TextClassificationOption{
			// go_emotions is multi-label: a message can be both "joy" and
			// "gratitude", so the scores are independent sigmoids rather than a
			// softmax that sums to one.
			pipelines.WithMultiLabel(),
			pipelines.WithSigmoid(),
			// See moodSequenceLength: this is the memory bound, not a tuning knob.
			pipelines.WithFixedPadding(moodSequenceLength),
		},
	}

	moodPipeline, moodErr = hugot.NewPipeline(session, config)
}

// ScoreMood returns the emotion distribution for a single input string. The
// pipeline is initialized (and the model downloaded) on first call.
//
// Unlike Embed there is no shrink-and-retry loop: fixed padding gives every
// input the same tensor shape and silently truncates anything longer, so an
// over-long input cannot fail to compile.
func ScoreMood(text string) (Mood, error) {
	moodOnce.Do(initMoodPipeline)
	if moodErr != nil {
		return Mood{}, moodErr
	}

	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return Mood{}, errors.New("cannot score empty text")
	}
	return runScore(string(runes[:min(len(runes), moodMaxRunes)]))
}

// runScore runs a single input through the classifier under the mood lock.
func runScore(text string) (Mood, error) {
	moodMu.Lock()
	defer moodMu.Unlock()

	out, err := moodPipeline.RunPipeline(context.Background(), []string{text})
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

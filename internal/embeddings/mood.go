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
		},
	}

	moodPipeline, moodErr = hugot.NewPipeline(session, config)
}

// ScoreMood returns the emotion distribution for a single input string. The
// pipeline is initialized (and the model downloaded) on first call.
//
// Input length is capped the same way Embed caps it: the Go-backend tokenizer
// does not reliably truncate, and this model's context window is no larger.
func ScoreMood(text string) (Mood, error) {
	moodOnce.Do(initMoodPipeline)
	if moodErr != nil {
		return Mood{}, moodErr
	}

	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return Mood{}, errors.New("cannot score empty text")
	}

	limit := min(len(runes), firstAttemptRunes)

	var lastErr error
	for {
		mood, err := runScore(string(runes[:limit]))
		if err == nil {
			return mood, nil
		}
		lastErr = err
		if limit <= minAttemptRunes {
			return Mood{}, lastErr
		}
		if limit /= 2; limit < minAttemptRunes {
			limit = minAttemptRunes
		}
	}
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

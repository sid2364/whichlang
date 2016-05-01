package neuralnet

import (
	"math"
	"math/rand"

	"github.com/unixpickle/whichlang/tokens"
)

const InitialIterationCount = 2000
const DefaultMaxIterations = 6400

// HiddenLayerScale specifies how much larger
// the hidden layer is than the output layer.
const HiddenLayerScale = 2.0

func Train(data map[string][]tokens.Freqs) *Network {
	ds := NewDataSet(data)

	var best *Network
	var bestCrossScore float64
	var bestTrainScore float64

	for stepPower := -20; stepPower < 10; stepPower++ {
		stepSize := math.Pow(2, float64(stepPower))

		t := NewTrainer(ds, stepSize)
		t.Train(DefaultMaxIterations)

		n := t.Network()
		if n.containsNaN() {
			continue
		}
		crossScore := ds.CrossScore(n)
		trainScore := ds.TrainingScore(n)
		if (crossScore == bestCrossScore && trainScore >= bestTrainScore) ||
			best == nil || (crossScore > bestCrossScore) {
			bestCrossScore = crossScore
			bestTrainScore = trainScore
			best = n
		}
	}

	return best
}

type Trainer struct {
	n *Network
	d *DataSet
	g *gradientCalc

	stepSize float64
}

func NewTrainer(d *DataSet, stepSize float64) *Trainer {
	hiddenCount := int(HiddenLayerScale * float64(len(d.TrainingSamples)))
	n := &Network{
		Tokens:        d.Tokens(),
		Langs:         d.Langs(),
		HiddenWeights: make([][]float64, hiddenCount),
		OutputWeights: make([][]float64, len(d.TrainingSamples)),
	}
	for i := range n.OutputWeights {
		n.OutputWeights[i] = make([]float64, hiddenCount+1)
		for j := range n.OutputWeights {
			n.OutputWeights[i][j] = rand.Float64()*2 - 1
		}
	}
	for i := range n.HiddenWeights {
		n.HiddenWeights[i] = make([]float64, len(n.Tokens)+1)
		for j := range n.HiddenWeights[i] {
			n.HiddenWeights[i][j] = rand.Float64()*2 - 1
		}
	}
	return &Trainer{
		n:        n,
		d:        d,
		g:        newGradientCalc(n),
		stepSize: stepSize,
	}
}

func (t *Trainer) Train(maxIters int) {
	iters := InitialIterationCount
	if iters > maxIters {
		iters = maxIters
	}
	for i := 0; i < iters; i++ {
		t.runAllSamples()
	}
	if iters == maxIters {
		return
	}

	if t.n.containsNaN() {
		return
	}

	// Use cross-validation to find the best
	// number of iterations.
	crossScore := t.d.CrossScore(t.n)
	trainScore := t.d.TrainingScore(t.n)
	lastNet := t.n.Copy()
	for {
		nextAmount := iters
		if nextAmount+iters > maxIters {
			nextAmount = maxIters - iters
		}
		for i := 0; i < nextAmount; i++ {
			t.runAllSamples()
		}
		iters += nextAmount

		if t.n.containsNaN() {
			break
		}

		newCrossScore := t.d.CrossScore(t.n)
		newTrainScore := t.d.TrainingScore(t.n)
		if (newCrossScore == crossScore && newTrainScore == trainScore) ||
			newCrossScore < crossScore {
			t.n = lastNet
			return
		}

		crossScore = newCrossScore
		trainScore = newTrainScore

		if iters == maxIters {
			return
		}
		lastNet = t.n.Copy()
	}
}

func (t *Trainer) Network() *Network {
	return t.n
}

func (t *Trainer) runAllSamples() {
	for i, lang := range t.n.Langs {
		samples := t.d.TrainingSamples[lang]
		for _, sample := range samples {
			t.descendSample(sample, i)
		}
	}
}

// descendSample performs gradient descent to
// reduce the output error for a given sample.
func (t *Trainer) descendSample(f tokens.Freqs, langIdx int) {
	t.g.Compute(f, langIdx)
	t.g.Normalize()

	for i, partials := range t.g.HiddenPartials {
		for j, partial := range partials {
			t.n.HiddenWeights[i][j] -= partial * t.stepSize
		}
	}
	for i, partials := range t.g.OutputPartials {
		for j, partial := range partials {
			t.n.OutputWeights[i][j] -= partial * t.stepSize
		}
	}
}
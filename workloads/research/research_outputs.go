package research

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"

	alloramath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/allora-network/allora-simulator/types"
	log "github.com/rs/zerolog/log"
)

// InitializeWorkerResearchParams generates research parameters for workers/inferers
func InitializeWorkerResearchParams(volatility float64) *types.ResearchParams {
	params := &types.ResearchParams{
		Volatility: volatility,
	}

	// errors = 10^(normal(log10(2.0*volatility), log10(1.5)))
	params.Error = math.Pow(10, rand.NormFloat64()*math.Log10(1.5)+math.Log10(2.0*volatility))
	// biasWithVolatility = normal(0, 0.5*volatility)
	params.BiasWithVolatility = rand.NormFloat64() * 0.5 * volatility
	// bias = normal(0, 0.3)
	params.Bias = rand.NormFloat64() * 0.3
	// contextSensitivity = sigmoid(10*(uniform(0,1)-0.5))
	raw := rand.Float64()
	params.ContextSensitivity = 1.0 / (1.0 + math.Exp(-10.0*(raw-0.5)))

	return params
}

// InitializeReputerResearchParams generates research parameters for reputers
func InitializeReputerResearchParams() *types.ResearchParams {
	params := &types.ResearchParams{}

	// logErrors = 10^(normal(log10(0.1), log10(1.25)))
	params.Error = math.Pow(10, rand.NormFloat64()*math.Log10(1.25)+math.Log10(0.1))
	// logBiases = normal(0, 0.05)
	params.Bias = rand.NormFloat64() * 0.05

	return params
}

// Experience factor: decays from 1 to 0.5 as age increases
func experienceFactor(config *types.ResearchConfig, age int) float64 {
	return config.BaseExperienceFactor * (1.0 + math.Exp(config.ExperienceGrowth*float64(age)))
}

func GetOutperformFactor(config *types.ResearchConfig, outperform bool) float64 {
	if outperform {
		return config.OutperformValue
	}
	return 1.0
}

// GetNextGroundTruth generates the next ground truth price
func GetNextGroundTruth(state *types.GroundTruthState, initialPrice float64, drift float64, volatility float64) *types.GroundTruthState {
	// Generate return from normal distribution
	returnT := rand.NormFloat64()*volatility + drift

	// Update state
	newState := &types.GroundTruthState{
		CumulativeReturn: state.CumulativeReturn + returnT,
		LastReturn:       returnT,
	}

	// Calculate new price
	newState.CurrentPrice = initialPrice * math.Exp(newState.CumulativeReturn)

	return newState
}

// Generates inferer output
func GetInfererOutput(
	config *types.ResearchConfig,
	groundTruth float64,
	error float64,
	bias float64,
	age int,
	outperformFlag bool,
) alloramath.BoundedExp40Dec {
	factor := GetOutperformFactor(config, outperformFlag)
	xp := experienceFactor(config, age)

	// Adjust error and bias
	adjustedError := factor * xp * error
	adjustedBias := factor * xp * bias

	// Generate random normal difference
	difference := rand.NormFloat64()*adjustedError + adjustedBias

	// Calculate prediction
	prediction := groundTruth + difference

	return alloramath.MustNewBoundedExp40DecFromString(fmt.Sprintf("%f", prediction))
}

// Generates forecaster output
func GetForecasterOutput(
	config *types.ResearchConfig,
	lossObs []LossObs,
	logError float64,
	logBias float64,
	contextSens float64,
	age int,
) []*emissionstypes.InputForecastElement {
	xp := experienceFactor(config, age)
	adjustedLogError := xp * logError
	adjustedLogBias := xp * logBias

	forecastElements := make([]*emissionstypes.InputForecastElement, 0)
	// Generate random log differences
	for _, loss := range lossObs {
		logDiff := rand.NormFloat64()*adjustedLogError + adjustedLogBias

		// Calculate no-outperformance loss
		lossNoOutperformance := loss.Loss
		if loss.Outperform {
			outperformFactorValue := GetOutperformFactor(config, true)
			lossNoOutperformance = loss.Loss / (outperformFactorValue * outperformFactorValue)
		}

		// Calculate predicted losses with and without context
		predictedLossNoContext := math.Pow(10, math.Log10(lossNoOutperformance)+logDiff)
		predictedLossContext := math.Pow(10, math.Log10(loss.Loss)+logDiff)

		// Combine using context sensitivity
		finalLoss := math.Pow(10,
			contextSens*math.Log10(predictedLossContext)+
				(1-contextSens)*math.Log10(predictedLossNoContext))

		// Check value is not NaN, Inf, or -Inf
		if math.IsNaN(finalLoss) || math.IsInf(finalLoss, 0) || finalLoss <= 0 {
			log.Error().Msgf("Invalid loss value: %f", finalLoss)
			continue
		}

		forecastElements = append(forecastElements, &emissionstypes.InputForecastElement{
			Inferer: loss.InfererAddr,
			Value:   alloramath.MustNewBoundedExp40DecFromString(fmt.Sprintf("%f", finalLoss)),
		})
	}

	return forecastElements
}

func GetReputerOutput(sourceTruth float64, vb *emissionstypes.ValueBundle, logError, logBias float64) (emissionstypes.InputValueBundle, error) {
	losses := emissionstypes.InputValueBundle{
		TopicId:             vb.TopicId,
		ReputerRequestNonce: vb.ReputerRequestNonce,
		Reputer:             vb.Reputer,
		ExtraData:           vb.ExtraData,
	}

	computeLoss := func(value alloramath.Dec) (alloramath.BoundedExp40Dec, error) {
		valueFloat, err := strconv.ParseFloat(value.String(), 64)
		if err != nil {
			return alloramath.BoundedExp40Dec{}, err
		}

		// Calculate base loss
		baseLoss := GetLosses(sourceTruth, valueFloat)

		// Apply log perturbation
		logDiff := rand.NormFloat64()*logError + logBias
		perturbedLoss := math.Pow(10, math.Log10(baseLoss)+logDiff)

		return alloramath.MustNewBoundedExp40DecFromString(fmt.Sprintf("%f", perturbedLoss)), nil
	}

	// Combined Value
	combinedLoss, err := computeLoss(vb.CombinedValue)
	if err != nil {
		return emissionstypes.InputValueBundle{}, err
	}
	losses.CombinedValue = combinedLoss

	// Naive Value
	naiveLoss, err := computeLoss(vb.NaiveValue)
	if err != nil {
		return emissionstypes.InputValueBundle{}, err
	}
	losses.NaiveValue = naiveLoss

	// Inferer Values
	losses.InfererValues = make([]*emissionstypes.InputWorkerAttributedValue, len(vb.InfererValues))
	for i, val := range vb.InfererValues {
		loss, err := computeLoss(val.Value)
		if err != nil {
			return emissionstypes.InputValueBundle{}, err
		}
		losses.InfererValues[i] = &emissionstypes.InputWorkerAttributedValue{Worker: val.Worker, Value: loss}
	}

	// Forecaster Values
	losses.ForecasterValues = make([]*emissionstypes.InputWorkerAttributedValue, len(vb.ForecasterValues))
	for i, val := range vb.ForecasterValues {
		loss, err := computeLoss(val.Value)
		if err != nil {
			return emissionstypes.InputValueBundle{}, err
		}
		losses.ForecasterValues[i] = &emissionstypes.InputWorkerAttributedValue{Worker: val.Worker, Value: loss}
	}

	// One Out Values
	losses.OneOutInfererValues = make([]*emissionstypes.InputWithheldWorkerAttributedValue, len(vb.OneOutInfererValues))
	for i, val := range vb.OneOutInfererValues {
		loss, err := computeLoss(val.Value)
		if err != nil {
			return emissionstypes.InputValueBundle{}, err
		}
		losses.OneOutInfererValues[i] = &emissionstypes.InputWithheldWorkerAttributedValue{Worker: val.Worker, Value: loss}
	}

	losses.OneOutForecasterValues = make([]*emissionstypes.InputWithheldWorkerAttributedValue, len(vb.OneOutForecasterValues))
	for i, val := range vb.OneOutForecasterValues {
		loss, err := computeLoss(val.Value)
		if err != nil {
			return emissionstypes.InputValueBundle{}, err
		}
		losses.OneOutForecasterValues[i] = &emissionstypes.InputWithheldWorkerAttributedValue{Worker: val.Worker, Value: loss}
	}

	losses.OneInForecasterValues = make([]*emissionstypes.InputWorkerAttributedValue, len(vb.OneInForecasterValues))
	for i, val := range vb.OneInForecasterValues {
		loss, err := computeLoss(val.Value)
		if err != nil {
			return emissionstypes.InputValueBundle{}, err
		}
		losses.OneInForecasterValues[i] = &emissionstypes.InputWorkerAttributedValue{Worker: val.Worker, Value: loss}
	}

	losses.OneOutInfererForecasterValues = make([]*emissionstypes.InputOneOutInfererForecasterValues, len(vb.OneOutInfererForecasterValues))
	for i, val := range vb.OneOutInfererForecasterValues {
		oneOutInfererValues := make([]*emissionstypes.InputWithheldWorkerAttributedValue, len(val.OneOutInfererValues))
		for j, infererVal := range val.OneOutInfererValues {
			loss, err := computeLoss(infererVal.Value)
			if err != nil {
				return emissionstypes.InputValueBundle{}, err
			}
			oneOutInfererValues[j] = &emissionstypes.InputWithheldWorkerAttributedValue{Worker: infererVal.Worker, Value: loss}
		}

		losses.OneOutInfererForecasterValues[i] = &emissionstypes.InputOneOutInfererForecasterValues{
			Forecaster:          val.Forecaster,
			OneOutInfererValues: oneOutInfererValues,
		}
	}

	return losses, nil
}

// LossMSE calculates Mean Squared Error between observed and predicted values
func LossMSE(yObs, yPred float64) float64 {
	return math.Pow(yObs-yPred, 2)
}

// GetLosses calculates the loss between observed and predicted values
// Simplified version that only handles MSE for regression
func GetLosses(yObs, yPred float64) float64 {
	return LossMSE(yObs, yPred)
}

// GetMeanLoss calculates the mean loss between observed and predicted losses
// yObs: actual losses for each predictor at time i
// yPred: aggregator's predicted losses for each predictor at time i
func GetMeanLoss(yObs []LossObs, yPred []*emissionstypes.ForecastElement) float64 {
	if len(yObs) != len(yPred) {
		panic("GetMeanLoss: length mismatch between observed and predicted values")
	}
	// Convert yPred to float64

	var sumLoss float64
	n := len(yObs)

	for i := 0; i < n; i++ {
		// Convert yPred to float64
		output, err := strconv.ParseFloat(yPred[i].String(), 64)
		if err != nil {
			panic(err)
		}
		sumLoss += LossMSE(yObs[i].Loss, output)
	}

	return sumLoss / float64(n)
}

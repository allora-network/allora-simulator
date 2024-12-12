package research

import (
	"math"
	"math/rand"

	"github.com/allora-network/allora-simulator/types"
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

// Helper functions
func experienceFactor(config *types.ResearchConfig, age int) float64 {
	return config.BaseExperienceFactor + config.ExperienceGrowth*float64(age)
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

// // Generates inferer output
// func GetInfererOutput(
// 	config *types.ResearchConfig,
// 	groundTruth float64,
// 	error float64,
// 	bias float64,
// 	age int,
// 	outperformFlag bool,
// ) alloraMath.Dec {
// 	factor := GetOutperformFactor(config, outperformFlag)
// 	xp := experienceFactor(config, age)

// 	// Adjust error and bias
// 	adjustedError := factor * xp * error
// 	adjustedBias := factor * xp * bias

// 	// Generate random normal difference
// 	difference := rand.NormFloat64()*adjustedError + adjustedBias

// 	// Calculate prediction
// 	prediction := groundTruth + difference

// 	return alloraMath.MustNewDecFromString(fmt.Sprintf("%f", prediction))
// }

// // Generates forecaster output
// func GetForecasterOutput(
// 	config *types.ResearchConfig,
// 	lossObs []float64,
// 	logError float64,
// 	logBias float64,
// 	contextSens float64,
// 	age int,
// 	outperformer int,
// ) []alloraMath.Dec {
// 	xp := experienceFactor(config, age)
// 	adjustedLogError := xp * logError
// 	adjustedLogBias := xp * logBias

// 	result := make([]alloraMath.Dec, len(lossObs))

// 	// Generate random log differences
// 	for i, loss := range lossObs {
// 		logDiff := rand.NormFloat64()*adjustedLogError + adjustedLogBias

// 		// Calculate no-outperformance loss
// 		lossNoOutperformance := loss
// 		if i == outperformer {
// 			outperformFactorValue := GetOutperformFactor(config, true)
// 			lossNoOutperformance = loss / (outperformFactorValue * outperformFactorValue)
// 		}

// 		// Calculate predicted losses with and without context
// 		predictedLossNoContext := math.Pow(10, math.Log10(lossNoOutperformance)+logDiff)
// 		predictedLossContext := math.Pow(10, math.Log10(loss)+logDiff)

// 		// Combine using context sensitivity
// 		finalLoss := math.Pow(10,
// 			contextSens*math.Log10(predictedLossContext)+
// 				(1-contextSens)*math.Log10(predictedLossNoContext))

// 		result[i] = alloraMath.MustNewDecFromString(fmt.Sprintf("%f", finalLoss))
// 	}

// 	return result
// }

// // Generates reputer output
// func GetReputerOutput(
// 	losses []float64,
// 	logError float64,
// 	logBias float64,
// ) []alloraMath.Dec {
// 	result := make([]alloraMath.Dec, len(losses))

// 	for i, loss := range losses {
// 		// Generate random log difference
// 		logDiff := rand.NormFloat64()*logError + logBias

// 		// Calculate estimated loss
// 		estimatedLoss := math.Pow(10, math.Log10(loss)+logDiff)

// 		result[i] = alloraMath.MustNewDecFromString(fmt.Sprintf("%f", estimatedLoss))
// 	}

// 	return result
// }

// // SetOutperformer sets the outperformer for the round
// func SetOutperformer(workers []*types.Actor) {
// 	// Randomly select an outperformer
// 	outperformer := rand.Intn(len(workers))
// 	workers[outperformer].ResearchParams.Outperform = true
// }

// // GetLosses calculates losses between observed and predicted values using specified loss function
// func GetLosses(yObs, yPred []float64, lossFunction string, deltaHuber, gammaFocal, alphaFocal float64) float64 {
// 	switch lossFunction {
// 	case "mse":
// 		return lossMSE(yObs, yPred)
// 	default:
// 		panic(fmt.Sprintf("Loss function %s not recognized", lossFunction))
// 	}
// }

// // Individual loss functions
// func lossMSE(yObs, yPred []float64) float64 {
// 	if len(yObs) != len(yPred) {
// 		panic("Observation and prediction arrays must have same length")
// 	}

// 	var loss float64
// 	for i := range yObs {
// 		diff := yObs[i] - yPred[i]
// 		loss = diff * diff
// 	}
// 	return loss
// }

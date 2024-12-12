package research

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"

	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/allora-network/allora-simulator/types"
)

type ResearchSimulationData struct {
	Faucet                       *types.Actor
	EpochLength                  int64
	Actors                       []*types.Actor
	RegisteredInferersByTopic    map[uint64][]*types.Actor
	RegisteredForecastersByTopic map[uint64][]*types.Actor
	RegisteredReputersByTopic    map[uint64][]*types.Actor
	FailOnErr                    bool
	Mu                           sync.RWMutex
	InfererSimulatedValues       map[uint64]map[string]*alloraMath.Dec
	InfererOutperformers         map[uint64]string
	ForecasterSimulatedValues    map[uint64]map[string][]*emissionstypes.ForecastElement
	ForecasterOutperformers      map[uint64]string
	ReputerSimulatedValues       map[uint64]map[string]*alloraMath.Dec
}

type Registration struct {
	TopicId uint64
	Actor   *types.Actor
}

type LossObs struct {
	InfererAddr string
	Loss        float64
	Outperform  bool
}

// Add an inferer registration to the simulation data
func (s *ResearchSimulationData) AddInfererRegistration(topicId uint64, actor *types.Actor) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.RegisteredInferersByTopic[topicId] = append(s.RegisteredInferersByTopic[topicId], actor)
}

// Add a forecaster registration to the simulation data
func (s *ResearchSimulationData) AddForecasterRegistration(topicId uint64, actor *types.Actor) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.RegisteredForecastersByTopic[topicId] = append(s.RegisteredForecastersByTopic[topicId], actor)
}

// Add a reputer registration to the simulation data
func (s *ResearchSimulationData) AddReputerRegistration(topicId uint64, actor *types.Actor) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.RegisteredReputersByTopic[topicId] = append(s.RegisteredReputersByTopic[topicId], actor)
}

// Get an actor object from an address
func (s *ResearchSimulationData) GetActorFromAddr(addr string) (*types.Actor, bool) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	for _, actor := range s.Actors {
		if actor.Addr == addr {
			return actor, true
		}
	}
	return nil, false
}

// Get all inferers for a topic
func (s *ResearchSimulationData) GetInferersForTopic(topicId uint64) []*types.Actor {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.RegisteredInferersByTopic[topicId]
}

// Get all forecasters for a topic
func (s *ResearchSimulationData) GetForecastersForTopic(topicId uint64) []*types.Actor {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.RegisteredForecastersByTopic[topicId]
}

// Get all reputers for a topic
func (s *ResearchSimulationData) GetReputersForTopic(topicId uint64) []*types.Actor {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.RegisteredReputersByTopic[topicId]
}

// Generate inferer simulated values for next epoch
func (s *ResearchSimulationData) GenerateInfererSimulatedValuesForNextEpoch(config *types.ResearchConfig, topicId uint64, numberOfActiveEpochs int64, groundTruthState *types.GroundTruthState) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	inferers := s.GetInferersForTopic(topicId)
	s.SetInfererOutperformer(topicId, inferers)

	for _, inferer := range inferers {
		simulatedValue := GetInfererOutput(
			config,
			groundTruthState.CurrentPrice,
			inferer.ResearchParams.Error,
			inferer.ResearchParams.Bias,
			int(numberOfActiveEpochs),
			inferer.Addr == s.InfererOutperformers[topicId],
		)
		s.InfererSimulatedValues[topicId][inferer.Addr] = &simulatedValue
	}
}

// SetOutperformer sets the outperformer for the round
func (s *ResearchSimulationData) SetInfererOutperformer(topicId uint64, inferers []*types.Actor) {
	// Randomly select an outperformer
	outperformer := rand.Intn(len(inferers))
	s.InfererOutperformers[topicId] = inferers[outperformer].Addr
}

// Generates inferer output
func GetInfererOutput(
	config *types.ResearchConfig,
	groundTruth float64,
	error float64,
	bias float64,
	age int,
	outperformFlag bool,
) alloraMath.Dec {
	factor := GetOutperformFactor(config, outperformFlag)
	xp := experienceFactor(config, age)

	// Adjust error and bias
	adjustedError := factor * xp * error
	adjustedBias := factor * xp * bias

	// Generate random normal difference
	difference := rand.NormFloat64()*adjustedError + adjustedBias

	// Calculate prediction
	prediction := groundTruth + difference

	return alloraMath.MustNewDecFromString(fmt.Sprintf("%f", prediction))
}

func (s *ResearchSimulationData) GenerateForecasterSimulatedValuesForNextEpoch(
	config *types.ResearchConfig,
	topicId uint64,
	numberOfActiveEpochs int64,
	groundTruthState *types.GroundTruthState,
) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	forecasters := s.GetForecastersForTopic(topicId)
	s.SetForecasterOutperformer(topicId, forecasters)

	// Get inferer simulated values
	infererSimulatedValues := s.InfererSimulatedValues[topicId]

	// Get losses
	lossObs := make([]LossObs, 0)
	for address, inferer := range infererSimulatedValues {
		// convert inferer to float64
		infererFloat := inferer.String()
		// convert infererFloat to float64
		infererFloat64, err := strconv.ParseFloat(infererFloat, 64)
		if err != nil {
			panic(err)
		}
		loss := GetLosses(
			groundTruthState.CurrentPrice,
			infererFloat64,
		)
		lossObs = append(lossObs, LossObs{
			InfererAddr: address,
			Loss:        loss,
		})
	}

	for _, forecaster := range forecasters {
		simulatedValue := GetForecasterOutput(
			config,
			lossObs,
			forecaster.ResearchParams.Error,
			forecaster.ResearchParams.Bias,
			forecaster.ResearchParams.ContextSensitivity,
			int(numberOfActiveEpochs),
		)
		s.ForecasterSimulatedValues[topicId][forecaster.Addr] = simulatedValue
	}
}

// SetOutperformer sets the outperformer for the round
func (s *ResearchSimulationData) SetForecasterOutperformer(topicId uint64, forecasters []*types.Actor) {
	// Randomly select an outperformer
	outperformer := rand.Intn(len(forecasters))
	s.ForecasterOutperformers[topicId] = forecasters[outperformer].Addr
}

// Generates forecaster output
func GetForecasterOutput(
	config *types.ResearchConfig,
	lossObs []LossObs,
	logError float64,
	logBias float64,
	contextSens float64,
	age int,
) []*emissionstypes.ForecastElement {
	xp := experienceFactor(config, age)
	adjustedLogError := xp * logError
	adjustedLogBias := xp * logBias

	forecastElements := make([]*emissionstypes.ForecastElement, 0)
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

		forecastElements = append(forecastElements, &emissionstypes.ForecastElement{
			Inferer: loss.InfererAddr,
			Value:   alloraMath.MustNewDecFromString(fmt.Sprintf("%f", finalLoss)),
		})
	}

	return forecastElements
}

// Generates reputer output
func GetReputerOutput(
	losses []float64,
	logError float64,
	logBias float64,
) []alloraMath.Dec {
	result := make([]alloraMath.Dec, len(losses))

	for i, loss := range losses {
		// Generate random log difference
		logDiff := rand.NormFloat64()*logError + logBias

		// Calculate estimated loss
		estimatedLoss := math.Pow(10, math.Log10(loss)+logDiff)

		result[i] = alloraMath.MustNewDecFromString(fmt.Sprintf("%f", estimatedLoss))
	}

	return result
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

func ComputeLossBundle(sourceTruth float64, vb *emissionstypes.ValueBundle, logError, logBias float64) (emissionstypes.ValueBundle, error) {
	losses := emissionstypes.ValueBundle{
		TopicId:             vb.TopicId,
		ReputerRequestNonce: vb.ReputerRequestNonce,
		Reputer:             vb.Reputer,
		ExtraData:           vb.ExtraData,
	}

	computeLoss := func(value alloraMath.Dec) (alloraMath.Dec, error) {
		valueFloat, err := strconv.ParseFloat(value.String(), 64)
		if err != nil {
			return alloraMath.Dec{}, err
		}

		// Calculate base loss
		baseLoss := GetLosses(sourceTruth, valueFloat)

		// Apply log perturbation
		logDiff := rand.NormFloat64()*logError + logBias
		perturbedLoss := math.Pow(10, math.Log10(baseLoss)+logDiff)

		return alloraMath.MustNewDecFromString(fmt.Sprintf("%f", perturbedLoss)), nil
	}

	// Combined Value
	combinedLoss, err := computeLoss(vb.CombinedValue)
	if err != nil {
		return emissionstypes.ValueBundle{}, err
	}
	losses.CombinedValue = combinedLoss

	// Naive Value
	naiveLoss, err := computeLoss(vb.NaiveValue)
	if err != nil {
		return emissionstypes.ValueBundle{}, err
	}
	losses.NaiveValue = naiveLoss

	// Inferer Values
	losses.InfererValues = make([]*emissionstypes.WorkerAttributedValue, len(vb.InfererValues))
	for i, val := range vb.InfererValues {
		loss, err := computeLoss(val.Value)
		if err != nil {
			return emissionstypes.ValueBundle{}, err
		}
		losses.InfererValues[i] = &emissionstypes.WorkerAttributedValue{Worker: val.Worker, Value: loss}
	}

	// Forecaster Values
	losses.ForecasterValues = make([]*emissionstypes.WorkerAttributedValue, len(vb.ForecasterValues))
	for i, val := range vb.ForecasterValues {
		loss, err := computeLoss(val.Value)
		if err != nil {
			return emissionstypes.ValueBundle{}, err
		}
		losses.ForecasterValues[i] = &emissionstypes.WorkerAttributedValue{Worker: val.Worker, Value: loss}
	}

	// One Out Values
	losses.OneOutInfererValues = make([]*emissionstypes.WithheldWorkerAttributedValue, len(vb.OneOutInfererValues))
	for i, val := range vb.OneOutInfererValues {
		loss, err := computeLoss(val.Value)
		if err != nil {
			return emissionstypes.ValueBundle{}, err
		}
		losses.OneOutInfererValues[i] = &emissionstypes.WithheldWorkerAttributedValue{Worker: val.Worker, Value: loss}
	}

	losses.OneOutForecasterValues = make([]*emissionstypes.WithheldWorkerAttributedValue, len(vb.OneOutForecasterValues))
	for i, val := range vb.OneOutForecasterValues {
		loss, err := computeLoss(val.Value)
		if err != nil {
			return emissionstypes.ValueBundle{}, err
		}
		losses.OneOutForecasterValues[i] = &emissionstypes.WithheldWorkerAttributedValue{Worker: val.Worker, Value: loss}
	}

	losses.OneInForecasterValues = make([]*emissionstypes.WorkerAttributedValue, len(vb.OneInForecasterValues))
	for i, val := range vb.OneInForecasterValues {
		loss, err := computeLoss(val.Value)
		if err != nil {
			return emissionstypes.ValueBundle{}, err
		}
		losses.OneInForecasterValues[i] = &emissionstypes.WorkerAttributedValue{Worker: val.Worker, Value: loss}
	}

	return losses, nil
}

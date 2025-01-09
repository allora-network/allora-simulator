package research

import (
	"math/rand"
	"strconv"
	"sync"

	alloramath "github.com/allora-network/allora-chain/math"
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
	InfererSimulatedValues       map[uint64]map[string]*alloramath.Dec
	InfererOutperformers         map[uint64]string
	ForecasterSimulatedValues    map[uint64]map[string][]*emissionstypes.ForecastElement
	ForecasterOutperformers      map[uint64]string
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

func (s *ResearchSimulationData) AddInfererRegistration(topicId uint64, actor *types.Actor) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.RegisteredInferersByTopic[topicId] = append(s.RegisteredInferersByTopic[topicId], actor)
}

func (s *ResearchSimulationData) AddForecasterRegistration(topicId uint64, actor *types.Actor) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.RegisteredForecastersByTopic[topicId] = append(s.RegisteredForecastersByTopic[topicId], actor)
}

func (s *ResearchSimulationData) AddReputerRegistration(topicId uint64, actor *types.Actor) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.RegisteredReputersByTopic[topicId] = append(s.RegisteredReputersByTopic[topicId], actor)
}

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

func (s *ResearchSimulationData) GetInferersForTopic(topicId uint64) []*types.Actor {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.RegisteredInferersByTopic[topicId]
}

func (s *ResearchSimulationData) GetForecastersForTopic(topicId uint64) []*types.Actor {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.RegisteredForecastersByTopic[topicId]
}

func (s *ResearchSimulationData) GetReputersForTopic(topicId uint64) []*types.Actor {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.RegisteredReputersByTopic[topicId]
}

func (s *ResearchSimulationData) GetInfererSimulatedValues(topicId uint64) map[string]*alloramath.Dec {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.InfererSimulatedValues[topicId]
}

func (s *ResearchSimulationData) GetInfererSimulatedValue(topicId uint64, addr string) *alloramath.Dec {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.InfererSimulatedValues[topicId][addr]
}

func (s *ResearchSimulationData) GetForecasterSimulatedValue(topicId uint64, addr string) []*emissionstypes.ForecastElement {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.ForecasterSimulatedValues[topicId][addr]
}

func (s *ResearchSimulationData) GetInfererOutperformer(topicId uint64) string {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.InfererOutperformers[topicId]
}

func (s *ResearchSimulationData) GetForecasterOutperformer(topicId uint64) string {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.ForecasterOutperformers[topicId]
}

func (s *ResearchSimulationData) SetInfererSimulatedValues(topicId uint64, values map[string]*alloramath.Dec) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.InfererSimulatedValues[topicId] = values
}

func (s *ResearchSimulationData) SetForecasterSimulatedValues(topicId uint64, values map[string][]*emissionstypes.ForecastElement) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.ForecasterSimulatedValues[topicId] = values
}

func (s *ResearchSimulationData) SetForecasterOutperformer(topicId uint64, forecasters []*types.Actor) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Randomly select an outperformer
	outperformer := rand.Intn(len(forecasters))
	s.ForecasterOutperformers[topicId] = forecasters[outperformer].Addr
}

func (s *ResearchSimulationData) SetInfererOutperformer(topicId uint64, inferers []*types.Actor) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Randomly select an outperformer
	outperformer := rand.Intn(len(inferers))
	s.InfererOutperformers[topicId] = inferers[outperformer].Addr
}

// Generate inferer simulated values for next epoch
func (s *ResearchSimulationData) GenerateInfererSimulatedValuesForNextEpoch(config *types.ResearchConfig, topicId uint64, numberOfActiveEpochs int64, groundTruthState *types.GroundTruthState) {
	inferers := s.GetInferersForTopic(topicId)
	s.SetInfererOutperformer(topicId, inferers)

	infererSimulatedValues := map[string]*alloramath.Dec{}
	for _, inferer := range inferers {
		outperformer := s.GetInfererOutperformer(topicId)
		simulatedValue := GetInfererOutput(
			&inferer.TxParams.Config.Research,
			groundTruthState.CurrentPrice,
			inferer.ResearchParams.Error,
			inferer.ResearchParams.Bias,
			int(numberOfActiveEpochs),
			inferer.Addr == outperformer,
		)
		infererSimulatedValues[inferer.Addr] = &simulatedValue
	}
	s.SetInfererSimulatedValues(topicId, infererSimulatedValues)
}

func (s *ResearchSimulationData) GenerateForecasterSimulatedValuesForNextEpoch(
	config *types.ResearchConfig,
	topicId uint64,
	numberOfActiveEpochs int64,
	groundTruthState *types.GroundTruthState,
) {
	forecasters := s.GetForecastersForTopic(topicId)
	s.SetForecasterOutperformer(topicId, forecasters)

	// Get inferer simulated values
	infererSimulatedValues := s.GetInfererSimulatedValues(topicId)

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

	forecasterSimulatedValues := map[string][]*emissionstypes.ForecastElement{}
	for _, forecaster := range forecasters {
		simulatedValue := GetForecasterOutput(
			config,
			lossObs,
			forecaster.ResearchParams.Error,
			forecaster.ResearchParams.Bias,
			forecaster.ResearchParams.ContextSensitivity,
			int(numberOfActiveEpochs),
		)
		forecasterSimulatedValues[forecaster.Addr] = simulatedValue
	}
	s.SetForecasterSimulatedValues(topicId, forecasterSimulatedValues)
}

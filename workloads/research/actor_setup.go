package research

import (
	"io"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"

	cosmosmath "cosmossdk.io/math"
	alloramath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/allora-network/allora-simulator/types"
	"github.com/allora-network/allora-simulator/workloads/common"
)

const stakeToAdd uint64 = 9e4

func CreateAndFundActors(
	config *types.Config,
	faucetMnemonic []byte,
	numActors int,
	epochLength int64,
	rand io.Reader,
) (
	faucet *types.Actor,
	simulationData *ResearchSimulationData,
) {
	faucet, actorsList, _ := common.CreateAndFundActors(config, faucetMnemonic, numActors, rand)

	data := ResearchSimulationData{
		Faucet:                       faucet,
		EpochLength:                  int64(epochLength),
		Actors:                       actorsList,
		RegisteredInferersByTopic:    map[uint64][]*types.Actor{},
		RegisteredForecastersByTopic: map[uint64][]*types.Actor{},
		RegisteredReputersByTopic:    map[uint64][]*types.Actor{},
		FailOnErr:                    false,
		Mu:                           sync.RWMutex{},
		InfererSimulatedValues:       make(map[uint64]map[string]*alloramath.BoundedExp40Dec),
		InfererOutperformers:         make(map[uint64]string),
		ForecasterSimulatedValues:    make(map[uint64]map[string][]*emissionstypes.InputForecastElement),
		ForecasterOutperformers:      make(map[uint64]string),
	}

	return faucet, &data
}

// RegisterWorkers registers numWorkers as workers in topicId
func RegisterWorkers(
	actors []*types.Actor,
	topicId uint64,
	data *ResearchSimulationData,
	numWorkers int,
	inferers bool,
) error {
	maxConcurrent := 1000
	sem := make(chan struct{}, maxConcurrent)
	completed := atomic.Int32{}

	var wg sync.WaitGroup
	log.Info().Msgf("Starting registration of %d workers in topic: %d", numWorkers, topicId)

	// Process all workers without batching
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		worker := actors[i]

		go func(worker *types.Actor, idx int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() {
				<-sem
				count := completed.Add(1)
				if int(count)%1000 == 0 || count == int32(numWorkers) {
					log.Info().Msgf("Processed %d/%d worker registrations (%.2f%%) for topic: %d",
						count, numWorkers,
						float64(count)/float64(numWorkers)*100,
						topicId,
					)
				}
			}()

			request := &emissionstypes.RegisterRequest{
				Sender:    worker.Addr,
				Owner:     worker.Addr,
				IsReputer: false,
				TopicId:   topicId,
			}

			_, updatedSeq, err := common.SendDataWithRetry(worker.TxParams, false, request)
			if err != nil {
				log.Error().Msgf("Error sending worker registration: %v", err.Error())
				return
			}
			worker.TxParams.Sequence = updatedSeq

			// Set the research params
			worker.ResearchParams = InitializeWorkerResearchParams(worker.TxParams.Config.Research.Volatility)

			if inferers {
				data.AddInfererRegistration(topicId, worker)
			} else {
				data.AddForecasterRegistration(topicId, worker)
			}
		}(worker, i)
	}

	wg.Wait()

	return nil
}

// RegisterReputersAndStake registers numReputers as reputers in topicId and stakes them
func RegisterReputersAndStake(
	actors []*types.Actor,
	topicId uint64,
	data *ResearchSimulationData,
	numReputers int,
) error {
	maxConcurrent := 1000
	sem := make(chan struct{}, maxConcurrent)
	completed := atomic.Int32{}

	var wg sync.WaitGroup
	log.Info().Msgf("Starting registration of %d reputers in topic: %d", numReputers, topicId)

	// Process all reputers without batching
	for i := 0; i < numReputers; i++ {
		wg.Add(1)
		reputer := actors[i]

		go func(reputer *types.Actor, idx int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() {
				<-sem
				count := completed.Add(1)
				if int(count)%100 == 0 || count == int32(numReputers) {
					log.Info().Msgf("Processed %d/%d reputer registrations (%.2f%%) for topic: %d",
						count, numReputers,
						float64(count)/float64(numReputers)*100,
						topicId,
					)
				}
			}()

			registerRequest := &emissionstypes.RegisterRequest{
				Sender:    reputer.Addr,
				Owner:     reputer.Addr,
				IsReputer: true,
				TopicId:   topicId,
			}
			stakeRequest := &emissionstypes.AddStakeRequest{
				Sender:  reputer.Addr,
				TopicId: topicId,
				Amount:  cosmosmath.NewIntFromUint64(stakeToAdd),
			}

			_, updatedSeq, err := common.SendDataWithRetry(reputer.TxParams, true, registerRequest, stakeRequest)
			if err != nil {
				log.Error().Msgf("Error sending reputer stake: %v", err.Error())
				return
			}
			reputer.TxParams.Sequence = updatedSeq

			// Set the research params
			reputer.ResearchParams = InitializeReputerResearchParams()

			data.AddReputerRegistration(topicId, reputer)
		}(reputer, i)
	}

	wg.Wait()

	return nil
}

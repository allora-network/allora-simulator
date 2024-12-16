package research

import (
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/allora-network/allora-simulator/lib"
	"github.com/allora-network/allora-simulator/transaction"
	"github.com/allora-network/allora-simulator/types"
)

func StartActorLoops(
	data *ResearchSimulationData,
	config *types.Config,
	topicIds []uint64,
) error {
	log.Printf("Starting submission loop for %d topics", len(topicIds))

	totalRoutines := len(topicIds) * 2 // 2 routines per topic (worker + reputer)
	errChan := make(chan error, totalRoutines)

	var wg sync.WaitGroup
	wg.Add(totalRoutines)

	for _, topicId := range topicIds {
		log.Printf("Starting submission loop for topic: %d", topicId)

		// Initialize research simulation data for topic

		// Start worker routine
		go func(tid uint64) {
			defer wg.Done()
			if err := runWorkersProcess(data, config, tid); err != nil {
				select {
				case errChan <- fmt.Errorf("worker routine failed for topic %d: %w", tid, err):
				default:
					log.Printf("Error channel full - worker error for topic %d: %v", tid, err)
				}
			}
		}(topicId)

		// Start reputer routine
		go func(tid uint64) {
			defer wg.Done()
			if err := runReputersProcess(data, config, tid); err != nil {
				select {
				case errChan <- fmt.Errorf("reputer routine failed for topic %d: %w", tid, err):
				default:
					log.Printf("Error channel full - reputer error for topic %d: %v", tid, err)
				}
			}
		}(topicId)
	}

	// Create a channel that closes when all goroutines are done
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for either completion or an error
	select {
	case err := <-errChan:
		return err
	case <-done:
		return nil
	case <-func() <-chan time.Time {
		if config.TimeoutMinutes == -1 {
			log.Printf("Timeout is disabled")
			return make(<-chan time.Time)
		}
		log.Printf("Timeout is enabled: %d minutes", config.TimeoutMinutes)
		return time.After(time.Duration(config.TimeoutMinutes) * time.Minute)
	}():
		return fmt.Errorf("simulation timed out after %d minutes", config.TimeoutMinutes)
	}
}

// Will check for nonce opened every 4s and if opened, will produce inferences and forecasts
func runWorkersProcess(
	data *ResearchSimulationData,
	config *types.Config,
	topicId uint64,
) error {
	numberOfActiveEpochs := int64(0)
	latestNonceHeightActedUpon := int64(0)
	groundTruthState := &types.GroundTruthState{
		CumulativeReturn: 0,
		CurrentPrice:     config.Research.InitialPrice,
		LastReturn:       0,
	}
	// Generate cold start epoch data
	data.GenerateInfererSimulatedValuesForNextEpoch(&config.Research, topicId, numberOfActiveEpochs, groundTruthState)
	data.GenerateForecasterSimulatedValuesForNextEpoch(&config.Research, topicId, numberOfActiveEpochs, groundTruthState)
	for {
		latestOpenInfererNonce, err := lib.GetLatestOpenWorkerNonceByTopicId(config, topicId) // TODO: Update this function name
		if err != nil {
			return err
		} else if latestOpenInfererNonce > latestNonceHeightActedUpon {
			log.Printf("Inferer nonce opened for topic: %d at height: %d", topicId, latestOpenInfererNonce)
			latestNonceHeightActedUpon = latestOpenInfererNonce

			// Get all inferers for the topic
			inferers := data.GetInferersForTopic(topicId)

			log.Printf("Building and committing inferer payload for topic: %d", topicId)
			wasError := createAndSendInfererPayloads(data, topicId, inferers, latestOpenInfererNonce)
			if wasError {
				log.Printf("Error building and committing inferer payload for topic: %d", topicId)
			}

			// Get all forecasters for the topic
			forecasters := data.GetForecastersForTopic(topicId)

			log.Printf("Building and committing forecaster payload for topic: %d", topicId)
			wasError = createAndSendForecasterPayloads(data, topicId, forecasters, latestOpenInfererNonce)
			if wasError {
				log.Printf("Error building and committing forecaster payload for topic: %d", topicId)
			}

			// Update ground truth state
			groundTruthState = GetNextGroundTruth(groundTruthState, config.Research.InitialPrice, config.Research.Drift, config.Research.Volatility)

			log.Printf("Successfully built and committed inferer payload for topic: %d for %v inferers", topicId, len(inferers))
			numberOfActiveEpochs++

			// Generate inferer and forecaster values for the next epoch
			data.GenerateInfererSimulatedValuesForNextEpoch(&config.Research, topicId, numberOfActiveEpochs, groundTruthState)
			data.GenerateForecasterSimulatedValuesForNextEpoch(&config.Research, topicId, numberOfActiveEpochs, groundTruthState)
		}
		time.Sleep(4 * time.Second)
	}
}

// Will check for nonce opened every 4s and if opened, will produce reputation
func runReputersProcess(
	data *ResearchSimulationData,
	config *types.Config,
	topicId uint64,
) error {
	latestNonceHeightActedUpon := int64(0)
	groundTruthState := &types.GroundTruthState{
		CumulativeReturn: 0,
		CurrentPrice:     config.Research.InitialPrice,
		LastReturn:       0,
	}
	for {
		latestOpenReputerNonce, err := lib.GetOldestReputerNonceByTopicId(config, topicId)
		if err != nil {
			log.Printf("Error getting latest open reputer nonce on topic - node availability issue?: %v", err)
		} else {
			if latestOpenReputerNonce > latestNonceHeightActedUpon {
				log.Printf("Reputer nonce opened for topic: %d at height: %d", topicId, latestOpenReputerNonce)
				latestNonceHeightActedUpon = latestOpenReputerNonce

				// Get all reputers for the topic
				reputers := data.GetReputersForTopic(topicId)

				log.Printf("Building and committing reputer payload for topic: %d", topicId)
				wasError := createAndSendReputerPayloads(config, topicId, reputers, latestOpenReputerNonce, groundTruthState)
				if wasError {
					log.Printf("Error building and committing reputer payload for topic: %d", topicId)
				}

				// Update ground truth state
				groundTruthState = GetNextGroundTruth(groundTruthState, config.Research.InitialPrice, config.Research.Drift, config.Research.Volatility)

				log.Printf("Successfully built and committed reputer payload for topic: %d for %v reputers", topicId, len(reputers))
			}
		}
		time.Sleep(4 * time.Second)
	}
}

// Create and send inferer payloads
func createAndSendInfererPayloads(
	data *ResearchSimulationData,
	topicId uint64,
	inferers []*types.Actor,
	infererNonce int64,
) bool {
	completed := atomic.Int32{}
	start := time.Now()

	log.Printf("Starting inferer payload creation for %d inferers in topic: %d", len(inferers), topicId)

	for _, inferer := range inferers {
		go func(inferer *types.Actor) {
			defer func() {
				count := completed.Add(1)
				if int(count)%1000 == 0 || count == int32(len(inferers)) {
					elapsed := time.Since(start)
					log.Printf("Processed %d/%d inferer payloads (%.2f%%) for topic: %d in %s",
						count, len(inferers),
						float64(count)/float64(len(inferers))*100,
						topicId,
						elapsed)
				}
			}()

			infererData, err := createInfererDataBundle(data, topicId, infererNonce, inferer)
			if err != nil {
				log.Printf("Error creating inferer data bundle: %v", err.Error())
				return
			}

			_, updatedSeq, err := transaction.SendDataWithRetry(inferer.TxParams, false, &emissionstypes.InsertWorkerPayloadRequest{
				Sender:           inferer.Addr,
				WorkerDataBundle: infererData,
			})
			if err != nil {
				log.Printf("Error sending inferer payload: %v", err.Error())
			}
			inferer.TxParams.Sequence = updatedSeq
		}(inferer)
	}

	totalTime := time.Since(start)
	log.Printf("Total inferer payload creation time: %s", totalTime)

	return false
}

// Create inferences for a inferer
func createInfererDataBundle(
	data *ResearchSimulationData,
	topicId uint64,
	blockHeight int64,
	inferer *types.Actor,
) (*emissionstypes.WorkerDataBundle, error) {
	// Get inferer simulated value
	infererSimulatedValue := data.InfererSimulatedValues[topicId][inferer.Addr]

	infererDataBundle := &emissionstypes.WorkerDataBundle{
		Worker: inferer.Addr,
		Nonce: &emissionstypes.Nonce{
			BlockHeight: blockHeight,
		},
		TopicId: topicId,
		InferenceForecastsBundle: &emissionstypes.InferenceForecastBundle{
			Inference: &emissionstypes.Inference{
				TopicId:     topicId,
				BlockHeight: blockHeight,
				Inferer:     inferer.Addr,
				Value:       *infererSimulatedValue,
				ExtraData:   nil,
				Proof:       "",
			},
			Forecast: nil,
		},
		InferencesForecastsBundleSignature: nil,
		Pubkey:                             "",
	}

	// Sign transaction
	src := make([]byte, 0)
	src, err := infererDataBundle.InferenceForecastsBundle.XXX_Marshal(src, true)
	if err != nil {
		return nil, err
	}
	sig, err := inferer.TxParams.PrivKey.Sign(src)
	if err != nil {
		return nil, err
	}

	workerPublicKeyBytes := inferer.TxParams.PubKey.Bytes()
	infererDataBundle.InferencesForecastsBundleSignature = sig
	infererDataBundle.Pubkey = hex.EncodeToString(workerPublicKeyBytes)

	return infererDataBundle, nil
}

// Create and send reputer payloads
func createAndSendReputerPayloads(
	config *types.Config,
	topicId uint64,
	reputers []*types.Actor,
	reputerNonce int64,
	groundTruthState *types.GroundTruthState,
) bool {
	completed := atomic.Int32{}

	log.Printf("Starting reputer payload creation for %d reputers in topic: %d", len(reputers), topicId)

	for _, reputer := range reputers {
		go func(reputer *types.Actor) {
			defer func() {
				count := completed.Add(1)
				if int(count)%1000 == 0 || count == int32(len(reputers)) {
					log.Printf("Processed %d/%d reputer payloads (%.2f%%) for topic: %d",
						count, len(reputers),
						float64(count)/float64(len(reputers))*100,
						topicId,
					)
				}
			}()

			valueBundle, err := createReputerValueBundle(config, topicId, reputer, reputerNonce, groundTruthState)
			if err != nil {
				log.Printf("Error creating reputer value bundle: %v", err.Error())
				return
			}

			_, updatedSeq, err := transaction.SendDataWithRetry(reputer.TxParams, false, &emissionstypes.InsertReputerPayloadRequest{
				Sender:             reputer.Addr,
				ReputerValueBundle: valueBundle,
			})
			if err != nil {
				log.Printf("Error sending reputer payload: %v", err.Error())
			}
			reputer.TxParams.Sequence = updatedSeq
		}(reputer)
	}

	return false
}

// Generate the same valueBundle for a reputer
func createReputerValueBundle(
	config *types.Config,
	topicId uint64,
	reputer *types.Actor,
	reputerNonce int64,
	groundTruthState *types.GroundTruthState,
) (*emissionstypes.ReputerValueBundle, error) {

	// Get Network Inferences
	networkInferences, err := lib.GetNetworkInferencesAtBlock(config, topicId, reputerNonce)
	if err != nil {
		return nil, err
	}

	// Get Reputer Losses
	lossBundle, err := GetReputerOutput(
		groundTruthState.CurrentPrice,
		networkInferences,
		reputer.ResearchParams.Error,
		reputer.ResearchParams.Bias,
	)
	if err != nil {
		return nil, err
	}

	lossBundle.TopicId = topicId
	lossBundle.Reputer = reputer.Addr
	lossBundle.ReputerRequestNonce = &emissionstypes.ReputerRequestNonce{
		ReputerNonce: &emissionstypes.Nonce{
			BlockHeight: reputerNonce,
		},
	}

	// Sign transaction
	src := make([]byte, 0)
	src, err = lossBundle.XXX_Marshal(src, true)
	if err != nil {
		return nil, err
	}
	sig, err := reputer.TxParams.PrivKey.Sign(src)
	if err != nil {
		return nil, err
	}

	// Create a InsertReputerPayloadRequest message
	reputerValueBundle := &emissionstypes.ReputerValueBundle{
		ValueBundle: &lossBundle,
		Signature:   sig,
		Pubkey:      hex.EncodeToString(reputer.TxParams.PubKey.Bytes()),
	}

	return reputerValueBundle, nil
}

// Create and send forecaster payloads
func createAndSendForecasterPayloads(
	data *ResearchSimulationData,
	topicId uint64,
	forecasters []*types.Actor,
	forecasterNonce int64,
) bool {
	completed := atomic.Int32{}
	start := time.Now()

	log.Printf("Starting forecaster payload creation for %d forecasters in topic: %d", len(forecasters), topicId)

	for _, forecaster := range forecasters {
		go func(forecaster *types.Actor) {
			defer func() {
				count := completed.Add(1)
				if int(count)%1000 == 0 || count == int32(len(forecasters)) {
					elapsed := time.Since(start)
					log.Printf("Processed %d/%d forecaster payloads (%.2f%%) for topic: %d in %s",
						count, len(forecasters),
						float64(count)/float64(len(forecasters))*100,
						topicId,
						elapsed)
				}
			}()

			workerData, err := createForecasterDataBundle(data, topicId, forecasterNonce, forecaster)
			if err != nil {
				log.Printf("Error creating forecaster data bundle: %v", err.Error())
				return
			}

			_, updatedSeq, err := transaction.SendDataWithRetry(forecaster.TxParams, false, &emissionstypes.InsertWorkerPayloadRequest{
				Sender:           forecaster.Addr,
				WorkerDataBundle: workerData,
			})
			if err != nil {
				log.Printf("Error sending forecaster payload: %v", err.Error())
			}
			forecaster.TxParams.Sequence = updatedSeq
		}(forecaster)
	}

	totalTime := time.Since(start)
	log.Printf("Total forecaster payload creation time: %s", totalTime)

	return false
}

// Create forecasts for a forecaster
func createForecasterDataBundle(
	data *ResearchSimulationData,
	topicId uint64,
	blockHeight int64,
	forecaster *types.Actor,
) (*emissionstypes.WorkerDataBundle, error) {
	// Get forecaster simulated values
	forecasterSimulatedValues := data.ForecasterSimulatedValues[topicId][forecaster.Addr]

	workerDataBundle := &emissionstypes.WorkerDataBundle{
		Worker: forecaster.Addr,
		Nonce: &emissionstypes.Nonce{
			BlockHeight: blockHeight,
		},
		TopicId: topicId,
		InferenceForecastsBundle: &emissionstypes.InferenceForecastBundle{
			Inference: nil,
			Forecast:  nil,
		},
		InferencesForecastsBundleSignature: nil,
		Pubkey:                             "",
	}

	workerDataBundle.InferenceForecastsBundle.Forecast = &emissionstypes.Forecast{
		TopicId:          topicId,
		BlockHeight:      blockHeight,
		Forecaster:       forecaster.Addr,
		ForecastElements: forecasterSimulatedValues,
		ExtraData:        nil,
	}

	// Sign transaction
	src := make([]byte, 0)
	src, err := workerDataBundle.InferenceForecastsBundle.XXX_Marshal(src, true)
	if err != nil {
		return nil, err
	}
	sig, err := forecaster.TxParams.PrivKey.Sign(src)
	if err != nil {
		return nil, err
	}

	workerPublicKeyBytes := forecaster.TxParams.PubKey.Bytes()
	workerDataBundle.InferencesForecastsBundleSignature = sig
	workerDataBundle.Pubkey = hex.EncodeToString(workerPublicKeyBytes)

	return workerDataBundle, nil
}

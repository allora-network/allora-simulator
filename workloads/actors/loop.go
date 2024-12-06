package actors

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/allora-network/allora-simulator/lib"
	"github.com/allora-network/allora-simulator/transaction"
	"github.com/allora-network/allora-simulator/types"
	simulation "github.com/allora-network/allora-simulator/workloads/common"
)

func StartActorLoops(
	data *simulation.SimulationData,
	config *types.Config,
	topicIds []uint64,
) error {
	log.Printf("Starting submission loop for %d topics", len(topicIds))

	totalRoutines := len(topicIds) * 2 // 2 routines per topic (worker + reputer)
	errChan := make(chan error, totalRoutines)

	// Create wait group to track all goroutines
	var wg sync.WaitGroup
	wg.Add(totalRoutines)

	// For each topic, start a worker routine and a reputer routine
	for _, topicId := range topicIds {
		log.Printf("Starting submission loop for topic: %d", topicId)
		// Start worker routine
		go func(tid uint64) {
			defer wg.Done()
			if err := runTopicWorkersLoop(data, config, tid); err != nil {
				select {
				case errChan <- fmt.Errorf("worker routine failed for topic %d: %w", tid, err):
				default: // Don't block if channel is full
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
				default: // Don't block if channel is full
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
func runTopicWorkersLoop(
	data *simulation.SimulationData,
	config *types.Config,
	topicId uint64,
) error {
	latestNonceHeightActedUpon := int64(0)
	for {
		latestOpenWorkerNonce, err := lib.GetLatestOpenWorkerNonceByTopicId(config, topicId)
		if err != nil {
			return err
		} else {
			if latestOpenWorkerNonce > latestNonceHeightActedUpon {
				log.Printf("Worker nonce opened for topic: %d at height: %d", topicId, latestOpenWorkerNonce)
				// previousActiveSetNonce will be used to get the active set of workers from previous epoch for the forecasts
				previousActiveSetNonce := latestNonceHeightActedUpon
				latestNonceHeightActedUpon = latestOpenWorkerNonce

				// Get all workers for the topic
				workers := data.GetWorkersForTopic(topicId)

				// Get the active set of workers from previous epoch for the forecasts
				previousActiveWorkersAddresses, err := lib.GetActiveWorkersForTopic(config, topicId, previousActiveSetNonce)
				if err != nil {
					return err
				}

				log.Printf("Building and committing worker payload for topic: %d", topicId)
				wasError := createAndSendWorkerPayloads(topicId, workers, latestOpenWorkerNonce, previousActiveWorkersAddresses)
				if wasError {
					log.Printf("Error building and committing worker payload for topic: %d", topicId)
				}
				log.Printf("Successfully built and committed worker payload for topic: %d for %v workers", topicId, len(workers))
			}
		}
		time.Sleep(4 * time.Second)
	}
}

// Will check for nonce opened every 4s and if opened, will produce reputation
func runReputersProcess(
	data *simulation.SimulationData,
	config *types.Config,
	topicId uint64,
) error {
	latestNonceHeightActedUpon := int64(0)
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

				// Get the active set of workers from actual epoch
				activeWorkersAddresses, err := lib.GetActiveWorkersForTopic(config, topicId, latestOpenReputerNonce)
				if err != nil {
					return err
				}

				log.Printf("Building and committing reputer payload for topic: %d", topicId)
				wasError := createAndSendReputerPayloads(topicId, reputers, activeWorkersAddresses, latestOpenReputerNonce)
				if wasError {
					log.Printf("Error building and committing reputer payload for topic: %d", topicId)
				}
				log.Printf("Successfully built and committed reputer payload for topic: %d for %v reputers", topicId, len(reputers))
			}
		}
		time.Sleep(4 * time.Second)
	}
}

// Create and send worker payloads
func createAndSendWorkerPayloads(
	topicId uint64,
	workers []*types.Actor,
	workerNonce int64,
	previousActiveInferersAddresses []string,
) bool {
	completed := atomic.Int32{}
	start := time.Now()

	log.Printf("Starting worker payload creation for %d workers in topic: %d", len(workers), topicId)

	for _, worker := range workers {
		go func(worker *types.Actor) {
			defer func() {
				count := completed.Add(1)
				if int(count)%1000 == 0 || count == int32(len(workers)) {
					elapsed := time.Since(start)
					log.Printf("Processed %d/%d worker payloads (%.2f%%) for topic: %d in %s",
						count, len(workers),
						float64(count)/float64(len(workers))*100,
						topicId,
						elapsed)
				}
			}()

			workerData, err := createWorkerDataBundle(topicId, workerNonce, worker, previousActiveInferersAddresses)
			if err != nil {
				log.Printf("Error creating worker data bundle: %v", err.Error())
				return
			}

			_, updatedSeq, err := transaction.SendDataWithRetry(worker.Params, false, &emissionstypes.InsertWorkerPayloadRequest{
				Sender:           worker.Addr,
				WorkerDataBundle: workerData,
			})
			if err != nil {
				log.Printf("Error sending worker payload: %v", err.Error())
			}
			worker.Params.Sequence = updatedSeq
		}(worker)
	}

	totalTime := time.Since(start)
	log.Printf("Total worker payload creation time: %s", totalTime)

	return false
}

// Create inferences and forecasts for a worker
func createWorkerDataBundle(
	topicId uint64,
	blockHeight int64,
	inferer *types.Actor,
	previousActiveInferersAddresses []string,
) (*emissionstypes.WorkerDataBundle, error) {
	workerDataBundle := &emissionstypes.WorkerDataBundle{
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
				Value:       alloraMath.NewDecFromInt64(int64(rand.Intn(300) + 3000)),
				ExtraData:   nil,
				Proof:       "",
			},
			Forecast: nil,
		},
		InferencesForecastsBundleSignature: nil,
		Pubkey:                             "",
	}

	forecastElements := make([]*emissionstypes.ForecastElement, 0)
	for _, previousActiveInfererAddress := range previousActiveInferersAddresses {
		forecastElements = append(forecastElements, &emissionstypes.ForecastElement{
			Inferer: previousActiveInfererAddress,
			Value:   alloraMath.NewDecFromInt64(int64(rand.Intn(51) + 50)),
		})
	}
	// If there are forecast elements, create a forecast
	if len(forecastElements) != 0 {
		workerDataBundle.InferenceForecastsBundle.Forecast = &emissionstypes.Forecast{
			TopicId:          topicId,
			BlockHeight:      blockHeight,
			Forecaster:       inferer.Addr,
			ForecastElements: forecastElements,
			ExtraData:        nil,
		}
	}

	// Sign transaction
	src := make([]byte, 0)
	src, err := workerDataBundle.InferenceForecastsBundle.XXX_Marshal(src, true)
	if err != nil {
		return nil, err
	}
	sig, err := inferer.Params.PrivKey.Sign(src)
	if err != nil {
		return nil, err
	}

	workerPublicKeyBytes := inferer.Params.PubKey.Bytes()
	workerDataBundle.InferencesForecastsBundleSignature = sig
	workerDataBundle.Pubkey = hex.EncodeToString(workerPublicKeyBytes)

	return workerDataBundle, nil
}

// Create and send reputer payloads
func createAndSendReputerPayloads(
	topicId uint64,
	reputers []*types.Actor,
	workers []string,
	workerNonce int64,
) bool {
	completed := atomic.Int32{}

	reputerNonce := &emissionstypes.Nonce{
		BlockHeight: workerNonce,
	}

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

			valueBundle, err := createReputerValueBundle(topicId, reputer, workers, reputerNonce)
			if err != nil {
				log.Printf("Error creating reputer value bundle: %v", err.Error())
				return
			}

			_, updatedSeq, err := transaction.SendDataWithRetry(reputer.Params, false, &emissionstypes.InsertReputerPayloadRequest{
				Sender:             reputer.Addr,
				ReputerValueBundle: valueBundle,
			})
			if err != nil {
				log.Printf("Error sending reputer payload: %v", err.Error())
			}
			reputer.Params.Sequence = updatedSeq
		}(reputer)
	}

	return false
}

// Generate the same valueBundle for a reputer
func createReputerValueBundle(
	topicId uint64,
	reputer *types.Actor,
	workers []string,
	reputerNonce *emissionstypes.Nonce,
) (*emissionstypes.ReputerValueBundle, error) {
	valueBundle := emissionstypes.ValueBundle{
		TopicId:                topicId,
		Reputer:                reputer.Addr,
		ExtraData:              nil,
		CombinedValue:          alloraMath.NewDecFromInt64(100),
		InfererValues:          generateWorkerAttributedValueLosses(workers, 3000, 3500),
		ForecasterValues:       generateWorkerAttributedValueLosses(workers, 50, 50),
		NaiveValue:             alloraMath.NewDecFromInt64(100),
		OneOutInfererValues:    generateWithheldWorkerAttributedValueLosses(workers, 50, 50),
		OneOutForecasterValues: generateWithheldWorkerAttributedValueLosses(workers, 50, 50),
		OneInForecasterValues:  generateWorkerAttributedValueLosses(workers, 50, 50),
		ReputerRequestNonce: &emissionstypes.ReputerRequestNonce{
			ReputerNonce: reputerNonce,
		},
		OneOutInfererForecasterValues: nil,
	}

	// Sign transaction
	src := make([]byte, 0)
	src, err := valueBundle.XXX_Marshal(src, true)
	if err != nil {
		return nil, err
	}
	sig, err := reputer.Params.PrivKey.Sign(src)
	if err != nil {
		return nil, err
	}

	// Create a InsertReputerPayloadRequest message
	reputerValueBundle := &emissionstypes.ReputerValueBundle{
		ValueBundle: &valueBundle,
		Signature:   sig,
		Pubkey:      hex.EncodeToString(reputer.Params.PubKey.Bytes()),
	}

	return reputerValueBundle, nil
}

// For every worker, generate a worker attributed value
func generateWorkerAttributedValueLosses(
	workers []string,
	lowLimit,
	sum int,
) []*emissionstypes.WorkerAttributedValue {
	values := make([]*emissionstypes.WorkerAttributedValue, 0)
	for _, worker := range workers {
		values = append(values, &emissionstypes.WorkerAttributedValue{
			Worker: worker,
			Value:  alloraMath.NewDecFromInt64(int64(rand.Intn(lowLimit) + sum)),
		})
	}
	return values
}

// For every worker, generate a withheld worker attribute value
func generateWithheldWorkerAttributedValueLosses(
	workers []string,
	lowLimit,
	sum int,
) []*emissionstypes.WithheldWorkerAttributedValue {
	values := make([]*emissionstypes.WithheldWorkerAttributedValue, 0)
	for _, worker := range workers {
		values = append(values, &emissionstypes.WithheldWorkerAttributedValue{
			Worker: worker,
			Value:  alloraMath.NewDecFromInt64(int64(rand.Intn(lowLimit) + sum)),
		})
	}
	return values
}

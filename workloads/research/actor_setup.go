package research

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	cosmosmath "cosmossdk.io/math"
	alloramath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/allora-network/allora-simulator/lib"
	"github.com/allora-network/allora-simulator/transaction"
	"github.com/allora-network/allora-simulator/types"
	"github.com/allora-network/allora-simulator/workloads/common"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const stakeToAdd uint64 = 9e4

func CreateAndFundActors(
	config *types.Config,
	faucetMnemonic []byte,
	numActors int,
	epochLength int64,
) (
	faucet *types.Actor,
	simulationData *ResearchSimulationData,
) {
	var err error
	// fund all actors from the faucet with some amount
	// give everybody the same amount of money to start with
	actorsList := createActors(numActors, config)

	privKey, pubKey, faucetAddr := common.GetPrivKey(config.Prefix, faucetMnemonic)

	faucet = &types.Actor{
		Name: "faucet",
		Addr: faucetAddr,
		TxParams: &types.TransactionParams{
			Config:   config,
			Sequence: 0,
			AccNum:   0,
			PrivKey:  privKey,
			PubKey:   pubKey,
		},
	}
	preFundAmount, err := getPreFundAmount(faucet, numActors)
	if err != nil {
		log.Fatal(err)
	}

	// Update faucet account number
	faucet.TxParams.Sequence, faucet.TxParams.AccNum, err = lib.GetAccountInfo(faucet.Addr, faucet.TxParams.Config)
	if err != nil {
		log.Fatal(err)
	}

	err = fundActors(
		faucet,
		actorsList,
		preFundAmount,
	)
	if err != nil {
		log.Fatal(err)
	}

	//Update account numbers
	for _, actor := range actorsList {
		actor.TxParams.Sequence, actor.TxParams.AccNum, err = lib.GetAccountInfo(actor.Addr, actor.TxParams.Config)
		if err != nil {
			log.Fatal(err)
		}
	}

	data := ResearchSimulationData{
		Faucet:                       faucet,
		EpochLength:                  int64(epochLength),
		Actors:                       actorsList,
		RegisteredInferersByTopic:    map[uint64][]*types.Actor{},
		RegisteredForecastersByTopic: map[uint64][]*types.Actor{},
		RegisteredReputersByTopic:    map[uint64][]*types.Actor{},
		FailOnErr:                    false,
		Mu:                           sync.RWMutex{},
		InfererSimulatedValues:       make(map[uint64]map[string]*alloramath.Dec),
		InfererOutperformers:         make(map[uint64]string),
		ForecasterSimulatedValues:    make(map[uint64]map[string][]*emissionstypes.ForecastElement),
		ForecasterOutperformers:      make(map[uint64]string),
	}

	return faucet, &data
}

// Create a new actor and register them in the node's account registry
func createNewActor(numActors int, config *types.Config) *types.Actor {
	actorName := types.GetActorName(numActors)
	privKey, pubKey, address := common.GeneratePrivKey()

	return &types.Actor{
		Name: actorName,
		Addr: address,
		TxParams: &types.TransactionParams{
			Config:   config,
			Sequence: 0,
			AccNum:   0,
			PrivKey:  privKey,
			PubKey:   pubKey,
		},
	}
}

// Create a list of actors both as a map and a slice, returns both
func createActors(numToCreate int, config *types.Config) []*types.Actor {
	actorsList := make([]*types.Actor, numToCreate)
	for i := 0; i < numToCreate; i++ {
		actorsList[i] = createNewActor(i, config)
	}
	return actorsList
}

// Fund every target address from the sender in amount coins
func fundActors(
	sender *types.Actor,
	targets []*types.Actor,
	amount cosmosmath.Int,
) error {
	batchSize := 2000
	completed := atomic.Int32{}

	log.Printf("Starting funding of %d actors", len(targets))

	for i := 0; i < len(targets); i += batchSize {
		end := i + batchSize
		if end > len(targets) {
			end = len(targets)
		}
		batch := targets[i:end]

		inputCoins := sdktypes.NewCoins(
			sdktypes.NewCoin(
				"uallo",
				amount.MulRaw(int64(len(batch))),
			),
		)
		outputCoins := sdktypes.NewCoins(
			sdktypes.NewCoin("uallo", amount),
		)

		outputs := make([]banktypes.Output, len(batch))
		names := make([]string, len(batch))
		for j, actor := range batch {
			names[j] = actor.Name
			outputs[j] = banktypes.Output{
				Address: actor.Addr,
				Coins:   outputCoins,
			}
		}

		sendMsg := &banktypes.MsgMultiSend{
			Inputs: []banktypes.Input{
				{
					Address: sender.Addr,
					Coins:   inputCoins,
				},
			},
			Outputs: outputs,
		}

		_, updatedSeq, err := transaction.SendDataWithRetry(sender.TxParams, true, sendMsg)
		if err != nil {
			log.Printf("Error sending worker registration: %v", err.Error())
			return err
		}
		sender.TxParams.Sequence = updatedSeq
		count := completed.Add(int32(len(batch)))
		if int(count)%1000 == 0 || count == int32(len(targets)) {
			log.Printf("Processed %d/%d funding operations (%.2f%%)",
				count, len(targets),
				float64(count)/float64(len(targets))*100,
			)
		}
	}

	return nil
}

// Get the amount of money to give each actor in the simulation
// Based on how much money the faucet currently has
func getPreFundAmount(
	faucet *types.Actor,
	numActors int,
) (cosmosmath.Int, error) {
	faucetBal, err := lib.GetAccountBalance(faucet.Addr, faucet.TxParams.Config)
	if err != nil {
		return cosmosmath.ZeroInt(), err
	}
	// divide by 10 so you can at least run 10 runs
	amountForThisRun := faucetBal.QuoRaw(int64(10))
	ret := amountForThisRun.QuoRaw(int64(numActors))
	if ret.Equal(cosmosmath.ZeroInt()) || ret.IsNegative() {
		return cosmosmath.ZeroInt(), fmt.Errorf(
			"not enough funds in faucet account to fund actors",
		)
	}
	return ret, nil
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
	log.Printf("Starting registration of %d workers in topic: %d\n", numWorkers, topicId)

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
					log.Printf("Processed %d/%d worker registrations (%.2f%%) for topic: %d\n",
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

			_, updatedSeq, err := transaction.SendDataWithRetry(worker.TxParams, false, request)
			if err != nil {
				log.Printf("Error sending worker registration: %v", err.Error())
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
	log.Printf("Starting registration of %d reputers in topic: %d\n", numReputers, topicId)

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
					log.Printf("Processed %d/%d reputer registrations (%.2f%%) for topic: %d\n",
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

			_, updatedSeq, err := transaction.SendDataWithRetry(reputer.TxParams, true, registerRequest, stakeRequest)
			if err != nil {
				log.Printf("Error sending reputer stake: %v", err.Error())
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

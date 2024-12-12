package stress

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"cosmossdk.io/math"
	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/allora-network/allora-simulator/lib"
	"github.com/allora-network/allora-simulator/transaction"
	"github.com/allora-network/allora-simulator/types"
	proto "github.com/cosmos/gogoproto/proto"
)

const topicFunds int64 = 1e6

// Creates multiple topics in a single broadcast or separate broadcasts
func CreateTopics(
	actor *types.Actor,
	numTopics int,
	epochLength int64,
	createTopicsSameBlock bool,
) ([]uint64, error) {
	log.Printf("Creating %d topics, same block: %t", numTopics, createTopicsSameBlock)

	// Get Next Block Id
	topicId, err := lib.GetNextTopicId(actor.TxParams.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to get block height: %w", err)
	}

	if createTopicsSameBlock {
		// Create all topics in one broadcast
		requests := make([]*emissionstypes.CreateNewTopicRequest, numTopics)
		topicIds := make([]uint64, numTopics)
		topicId := topicId
		for i := 0; i < numTopics; i++ {
			requests[i] = &emissionstypes.CreateNewTopicRequest{
				Creator:                  actor.Addr,
				Metadata:                 fmt.Sprintf("Created topic %d", i+1),
				LossMethod:               "mse",
				EpochLength:              epochLength,
				GroundTruthLag:           epochLength,
				WorkerSubmissionWindow:   10,
				PNorm:                    alloraMath.NewDecFromInt64(3),
				AlphaRegret:              alloraMath.MustNewDecFromString("0.1"),
				AllowNegative:            false,
				Epsilon:                  alloraMath.MustNewDecFromString("0.01"),
				MeritSortitionAlpha:      alloraMath.MustNewDecFromString("0.1"),
				ActiveInfererQuantile:    alloraMath.MustNewDecFromString("0.25"),
				ActiveForecasterQuantile: alloraMath.MustNewDecFromString("0.25"),
				ActiveReputerQuantile:    alloraMath.MustNewDecFromString("0.25"),
			}
			topicIds[i] = topicId
			topicId++
		}

		protoMsgs := make([]proto.Message, len(requests))
		for i, req := range requests {
			protoMsgs[i] = req
		}

		_, updatedSeq, err := transaction.SendDataWithRetry(actor.TxParams, false, protoMsgs...)
		if err != nil {
			return nil, fmt.Errorf("failed to broadcast create topic requests: %w", err)
		}
		actor.TxParams.Sequence = updatedSeq
		log.Printf("Created topics: %v", topicIds)
		return topicIds, nil

	} else {
		// Create topics in separate broadcasts
		topicIds := make([]uint64, numTopics)
		for i := 0; i < numTopics; i++ {
			request := &emissionstypes.CreateNewTopicRequest{
				Creator:                  actor.Addr,
				Metadata:                 fmt.Sprintf("Created topic %d", i+1),
				LossMethod:               "mse",
				EpochLength:              epochLength,
				GroundTruthLag:           epochLength,
				WorkerSubmissionWindow:   10,
				PNorm:                    alloraMath.NewDecFromInt64(3),
				AlphaRegret:              alloraMath.MustNewDecFromString("0.1"),
				AllowNegative:            true,
				Epsilon:                  alloraMath.MustNewDecFromString("0.01"),
				MeritSortitionAlpha:      alloraMath.MustNewDecFromString("0.1"),
				ActiveInfererQuantile:    alloraMath.MustNewDecFromString("0.2"),
				ActiveForecasterQuantile: alloraMath.MustNewDecFromString("0.2"),
				ActiveReputerQuantile:    alloraMath.MustNewDecFromString("0.2"),
			}

			_, updatedSeq, err := transaction.SendDataWithRetry(actor.TxParams, true, request)
			if err != nil {
				return nil, fmt.Errorf("failed to broadcast create topic request %d: %w", i, err)
			}
			actor.TxParams.Sequence = updatedSeq

			topicIds[i] = topicId
			topicId++

			// wait a random amount of time between 4 and 20 seconds
			// try to variate nonce opennings
			waitTime := rand.Intn(16) + 4
			time.Sleep(time.Duration(waitTime) * time.Second)
		}

		log.Printf("Created topics: %v", topicIds)
		return topicIds, nil
	}
}

// broadcast a tx to fund a topic
func FundTopics(
	actor *types.Actor,
	topicIds []uint64,
) error {
	requests := make([]*emissionstypes.FundTopicRequest, len(topicIds))
	for i, topicId := range topicIds {
		requests[i] = &emissionstypes.FundTopicRequest{
			Sender:  actor.Addr,
			TopicId: topicId,
			Amount:  math.NewInt(topicFunds),
		}
		log.Printf("Funding topic: %d with amount: %d from: %s", topicId, topicFunds, actor.Addr)
	}

	protoMsgs := make([]proto.Message, len(requests))
	for i, req := range requests {
		protoMsgs[i] = req
	}

	_, updatedSeq, err := transaction.SendDataWithRetry(actor.TxParams, true, protoMsgs...)
	if err != nil {
		return fmt.Errorf("failed to broadcast fund topic requests: %w", err)
	}
	actor.TxParams.Sequence = updatedSeq

	return nil
}

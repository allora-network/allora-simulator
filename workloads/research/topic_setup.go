package research

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"cosmossdk.io/math"
	alloramath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/allora-network/allora-simulator/lib"
	"github.com/allora-network/allora-simulator/transaction"
	"github.com/allora-network/allora-simulator/types"
)

const topicFunds int64 = 1e6

func CreateAndFundResearchTopic(
	actor *types.Actor,
	config *types.Config,
) (uint64, error) {
	// Get Next Topic Id
	topicId, err := lib.GetNextTopicId(config)
	if err != nil {
		return 0, fmt.Errorf("failed to get topic id: %w", err)
	}

	request := &emissionstypes.CreateNewTopicRequest{
		Creator:                  actor.Addr,
		Metadata:                 "Research Topic",
		LossMethod:               config.Research.Topic.LossMethod,
		EpochLength:              config.Research.Topic.EpochLength,
		GroundTruthLag:           config.Research.Topic.GroundTruthLag,
		WorkerSubmissionWindow:   config.Research.Topic.WorkerSubmissionWindow,
		PNorm:                    alloramath.MustNewDecFromString(config.Research.Topic.PNorm),
		AlphaRegret:              alloramath.MustNewDecFromString(config.Research.Topic.AlphaRegret),
		AllowNegative:            config.Research.Topic.AllowNegative,
		Epsilon:                  alloramath.MustNewDecFromString(config.Research.Topic.Epsilon),
		MeritSortitionAlpha:      alloramath.MustNewDecFromString(config.Research.Topic.MeritSortitionAlpha),
		ActiveInfererQuantile:    alloramath.MustNewDecFromString(config.Research.Topic.ActiveInfererQuantile),
		ActiveForecasterQuantile: alloramath.MustNewDecFromString(config.Research.Topic.ActiveForecasterQuantile),
		ActiveReputerQuantile:    alloramath.MustNewDecFromString(config.Research.Topic.ActiveReputerQuantile),
		EnableWorkerWhitelist:    false,
		EnableReputerWhitelist:   false,
	}

	_, updatedSeq, err := transaction.SendDataWithRetry(actor.TxParams, true, request)
	if err != nil {
		return 0, fmt.Errorf("failed to create topic: %w", err)
	}
	actor.TxParams.Sequence = updatedSeq

	// Fund the topic
	fundRequest := &emissionstypes.FundTopicRequest{
		Sender:  actor.Addr,
		TopicId: topicId,
		Amount:  math.NewInt(topicFunds),
	}

	_, updatedSeq, err = transaction.SendDataWithRetry(actor.TxParams, true, fundRequest)
	if err != nil {
		return 0, fmt.Errorf("failed to fund topic: %w", err)
	}
	actor.TxParams.Sequence = updatedSeq

	log.Info().Msgf("Created and funded topic: %d", topicId)
	return topicId, nil
}

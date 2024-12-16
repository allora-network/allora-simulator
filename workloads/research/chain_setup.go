package research

import (
	"fmt"
	"log"

	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/allora-network/allora-simulator/transaction"
	"github.com/allora-network/allora-simulator/types"
)

// ConfigureChainParams sets up the chain parameters for research simulation
func ConfigureChainParams(actor *types.Actor, config *types.Config) error {
	log.Printf("Configuring chain parameters for research simulation")

	updateParamRequest := &emissionstypes.UpdateParamsRequest{
		Sender: actor.Addr,
		Params: &emissionstypes.OptionalParams{
			MaxSamplesToScaleScores: []uint64{config.Research.GlobalParams.MaxSamplesToScaleScores},
		},
	}

	_, updatedSeq, err := transaction.SendDataWithRetry(actor.TxParams, true, updateParamRequest)
	if err != nil {
		return fmt.Errorf("failed to update chain parameters: %w", err)
	}
	actor.TxParams.Sequence = updatedSeq

	log.Printf("Successfully configured chain parameters")
	return nil
}

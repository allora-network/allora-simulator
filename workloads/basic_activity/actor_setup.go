package basic_activity

import (
	"io"

	"github.com/allora-network/allora-simulator/types"
	"github.com/allora-network/allora-simulator/workloads/common"
)

func CreateAndFundActors(config *types.Config, faucetMnemonic []byte, rand io.Reader) *State {
	_, actorsList, fundedAmount := common.CreateAndFundActors(config, faucetMnemonic, config.BasicActivity.NumActors, rand)
	return NewState(actorsList, fundedAmount)
}

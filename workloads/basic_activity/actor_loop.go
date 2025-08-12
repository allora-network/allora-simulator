package basic_activity

import (
	"sync"

	"cosmossdk.io/math"
	"github.com/allora-network/allora-simulator/types"
	"github.com/allora-network/allora-simulator/workloads/common"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/rs/zerolog/log"
)

func Start(config *types.Config, state *State) error {
	log.Info().Int("nbActors", len(state.actors)).Msg("Starting basic activity simulation")

	for {
		actors := state.getShuffledActors()
		txCount := config.BasicActivity.TxsPerBlock.RandInBetween()
		log.Info().Uint32("txCount", txCount).Msg("Starting a new tx batch")

		var toRefund []*types.Actor
		sends := make(map[string]banktypes.MsgSend, txCount)
		for t := uint32(0); t < txCount; t++ {
			sendAmount := config.BasicActivity.SendAmount.RandInBetween()

			for i, a := range actors {
				if state.balances[a.Addr].GT(sendAmount) {
					sends[a.Addr] = banktypes.MsgSend{
						FromAddress: a.Addr,
						ToAddress:   state.pickRandomActorExcept(a.Addr).Addr,
						Amount:      sdktypes.NewCoins(sdktypes.NewCoin(config.Denom, sendAmount)),
					}
				} else {
					toRefund = append(toRefund, a)
				}

				actors = append(actors[:i], actors[i+1:]...)
			}
		}

		log.Info().Int("txCount", len(sends)).Int("refundCount", len(toRefund)).Msg("Sending transactions")
		var wg sync.WaitGroup

		wg.Add(1)
		go refundActors(state.faucet, toRefund, config.BasicActivity.RefundAmount, &wg)

		for addr, msg := range sends {
			actor := state.actorsPerAddr[addr]
			wg.Add(1)
			go sendTx(config, state, &wg, actor, msg)
		}

		wg.Wait()
	}
}

func sendTx(config *types.Config, state *State, wg *sync.WaitGroup, actor *types.Actor, msg banktypes.MsgSend) {
	log.Info().Str("from", actor.Addr).Str("to", msg.ToAddress).Str("amount", msg.Amount.String()).Msg("Sending transaction")
	res, updatedSeq, err := common.SendDataWithRetry(actor.TxParams, true, &msg)
	if err != nil {
		lEvt := log.Err(err).Str("addr", actor.Addr).Str("amount", msg.Amount.String())
		if res != nil {
			lEvt.Uint32("txCode", res.Code).
				Str("txCodespace", res.Codespace).
				Str("txLog", res.Log).
				Str("txHash", res.Hash.String())
		}
		lEvt.Msg("Could not send tx")
	} else if res.Code == 0 {
		state.decreaseActorBalance(actor.Addr, msg.Amount.AmountOf(config.Denom))
	}
	actor.TxParams.Sequence = updatedSeq
	wg.Done()
}

func refundActors(faucet *types.Actor, actors []*types.Actor, amount math.Int, wg *sync.WaitGroup) {
	if err := common.FundActors(faucet, actors, amount); err != nil {
		log.Error().Err(err).Msg("Failed to refund actors")
	}
	wg.Done()
}

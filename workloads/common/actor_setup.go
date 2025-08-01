package common

import (
	"fmt"
	"sync/atomic"

	cosmosmath "cosmossdk.io/math"
	"github.com/allora-network/allora-simulator/lib"
	"github.com/allora-network/allora-simulator/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/rs/zerolog/log"
)

func CreateAndFundActors(
	config *types.Config,
	faucetMnemonic []byte,
	numActors int,
) (
	faucet *types.Actor,
	actorsList []*types.Actor,
) {
	var err error
	// fund all actors from the faucet with some amount
	// give everybody the same amount of money to start with
	actorsList = createActors(numActors, config)

	privKey, pubKey, faucetAddr := GetPrivKey(config.Prefix, faucetMnemonic)

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
		log.Fatal().Err(err).Msgf("Failed to get pre-fund amount")
	}

	// Update faucet account number
	faucet.TxParams.Sequence, faucet.TxParams.AccNum, err = lib.GetAccountInfo(faucet.Addr, faucet.TxParams.Config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to get account info")
	}

	err = fundActors(
		faucet,
		actorsList,
		preFundAmount,
	)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to fund actors")
	}

	//Update account numbers
	for _, actor := range actorsList {
		actor.TxParams.Sequence, actor.TxParams.AccNum, err = lib.GetAccountInfo(actor.Addr, actor.TxParams.Config)
		if err != nil {
			log.Fatal().Err(err).Msgf("Failed to get account info")
		}
	}

	return faucet, actorsList
}

// Create a new actor and register them in the node's account registry
func createNewActor(numActors int, config *types.Config) *types.Actor {
	actorName := types.GetActorName(numActors)
	privKey, pubKey, address := GeneratePrivKey()

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
	batchSize := 1000
	completed := atomic.Int32{}

	log.Info().Msgf("Starting funding of %d actors", len(targets))

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

		_, updatedSeq, err := SendDataWithRetry(sender.TxParams, true, sendMsg)
		if err != nil {
			log.Error().Err(err).Msgf("Error sending worker registration: %v", err.Error())
			return err
		}
		sender.TxParams.Sequence = updatedSeq
		count := completed.Add(int32(len(batch)))
		if int(count)%1000 == 0 || count == int32(len(targets)) {
			log.Info().Msgf("Processed %d/%d funding operations (%.2f%%)",
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

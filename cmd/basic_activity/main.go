package main

import (
	"encoding/json"
	"math/rand"
	"os"

	"github.com/allora-network/allora-simulator/lib"
	"github.com/allora-network/allora-simulator/lib/logger"
	"github.com/allora-network/allora-simulator/types"
	"github.com/allora-network/allora-simulator/workloads/basic_activity"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog/log"
)

func main() {
	logger.InitLogger()
	log.Info().Msgf("Starting basic activity simulation...")

	config := types.Config{}
	data, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to read config file: %v", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse config: %v", err)
	}

	mnemonic, err := os.ReadFile("scripts/seedphrase")
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to read seed phrase: %v", err)
	}

	// Set Bech32 prefixes and seal the configuration once
	sdkConfig := sdk.GetConfig()
	sdkConfig.SetBech32PrefixForAccount(config.Prefix, config.Prefix+"pub")
	sdkConfig.SetBech32PrefixForValidator(config.Prefix+"valoper", config.Prefix+"valoperpub")
	sdkConfig.SetBech32PrefixForConsensusNode(config.Prefix+"valcons", config.Prefix+"valconspub")
	sdkConfig.Seal()

	// Set initial gas price before sending any transactions
	gasPrice, err := lib.GetGasPrice(&config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error getting base fee: %v", err)
	}
	lib.SetCurrentGasPrice(gasPrice)

	rnd := rand.New(rand.NewSource(config.BasicActivity.RandWalletSeed))
	state := basic_activity.CreateAndFundActors(&config, mnemonic, rnd)

	if err := basic_activity.Start(&config, state); err != nil {
		log.Fatal().Err(err).Msg("An error occured running basic activity simulation")
	}
}

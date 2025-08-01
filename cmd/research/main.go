package main

import (
	"encoding/json"
	"math/rand"
	"os"
	"time"

	"github.com/allora-network/allora-simulator/lib"
	"github.com/allora-network/allora-simulator/lib/logger"
	"github.com/allora-network/allora-simulator/types"
	"github.com/allora-network/allora-simulator/workloads/research"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/rs/zerolog/log"
)

func main() {
	logger.InitLogger()
	log.Info().Msgf("Starting research simulation...")

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

	// Calculate total number of actors
	totalActors := config.InferersPerTopic + config.ForecastersPerTopic + config.ReputersPerTopic

	// Set initial gas price before sending any transactions
	gasPrice, err := lib.GetGasPrice(&config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error getting base fee: %v", err)
	}
	lib.SetCurrentGasPrice(gasPrice)

	log.Info().Msgf("Creating and funding %d actors...", totalActors)
	faucet, simulationData := research.CreateAndFundActors(
		&config,
		mnemonic,
		totalActors,
		config.Research.Topic.EpochLength,
		rand.New(rand.NewSource(time.Now().UnixNano())),
	)
	log.Info().Msgf("Successfully created and funded all actors")

	// Configure chain global parameters
	err = research.ConfigureChainParams(faucet, &config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to configure chain parameters: %v", err)
	}

	log.Info().Msgf("Creating research topic...")
	topicId, err := research.CreateAndFundResearchTopic(faucet, &config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to create research topic: %v", err)
	}
	log.Info().Msgf("Successfully created research topic with ID: %d", topicId)

	log.Info().Msgf("Dividing actors into their respective roles...")
	// Divide actors into their roles
	startIdx := 0
	inferers := simulationData.Actors[startIdx : startIdx+config.InferersPerTopic]
	startIdx += config.InferersPerTopic

	forecasters := simulationData.Actors[startIdx : startIdx+config.ForecastersPerTopic]
	startIdx += config.ForecastersPerTopic

	reputers := simulationData.Actors[startIdx : startIdx+config.ReputersPerTopic]
	log.Info().Msgf("Actor roles assigned - Inferers: %d, Forecasters: %d, Reputers: %d",
		len(inferers), len(forecasters), len(reputers))

	// Register actors with delays between registrations
	time.Sleep(20 * time.Second)
	log.Info().Msgf("Starting reputer registration process (%d reputers)...", len(reputers))
	err = research.RegisterReputersAndStake(
		reputers,
		topicId,
		simulationData,
		config.ReputersPerTopic,
	)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error registering reputers: %v", err)
	}
	log.Info().Msgf("Successfully registered all reputers")

	time.Sleep(20 * time.Second)
	log.Info().Msgf("Starting inferer registration process (%d inferers)...", len(inferers))
	err = research.RegisterWorkers(
		inferers,
		topicId,
		simulationData,
		config.InferersPerTopic,
		true,
	)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error registering inferers: %v", err)
	}
	log.Info().Msgf("Successfully registered all inferers")

	time.Sleep(20 * time.Second)
	log.Info().Msgf("Starting forecaster registration process (%d forecasters)...", len(forecasters))
	err = research.RegisterWorkers(
		forecasters,
		topicId,
		simulationData,
		config.ForecastersPerTopic,
		false,
	)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error registering forecasters: %v", err)
	}
	log.Info().Msgf("Successfully registered all forecasters")

	// Start the simulation loops
	log.Info().Msgf("Initiating actor simulation loops...")
	err = research.StartActorLoops(
		simulationData,
		&config,
		[]uint64{topicId},
	)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error starting actor loops: %v", err)
	}
	log.Info().Msgf("Simulation loops started successfully")
}

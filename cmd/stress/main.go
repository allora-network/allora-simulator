package main

import (
	"encoding/json"
	"os"
	"time"

	"github.com/allora-network/allora-simulator/lib/logger"
	"github.com/allora-network/allora-simulator/types"
	"github.com/allora-network/allora-simulator/workloads/stress"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog/log"
)

func main() {
	logger.InitLogger()
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

	workersPerTopic := config.InferersPerTopic + config.ForecastersPerTopic
	numActors := (workersPerTopic + config.ReputersPerTopic) * config.NumTopics
	faucet, simulationData := stress.CreateAndFundActors(
		&config,
		mnemonic,
		numActors,
		config.EpochLength,
	)

	// Create topics
	topicIds, err := stress.CreateTopics(
		faucet,
		config.NumTopics,
		config.EpochLength,
		config.CreateTopicsSameBlock,
	)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to create topics: %v", err)
	}

	// Calculate actors per topic
	actorsPerTopic := workersPerTopic + config.ReputersPerTopic

	// Register actors
	for i, topicId := range topicIds {
		// Get the slice of actors for this topic
		startIdx := i * actorsPerTopic
		topicActors := simulationData.Actors[startIdx : startIdx+actorsPerTopic]

		workers := topicActors[:workersPerTopic]
		reputers := topicActors[workersPerTopic:]

		time.Sleep(20 * time.Second)
		log.Info().Msgf("Registering reputers and adding stake in  topic: %d", topicId)
		err = stress.RegisterReputersAndStake(
			reputers,
			topicId,
			simulationData,
			config.ReputersPerTopic,
		)
		if err != nil {
			log.Error().Err(err).Msgf("Error registering reputers: %v", err)
		}
		time.Sleep(20 * time.Second)
		log.Info().Msgf("Registering workers in  topic: %d", topicId)
		err = stress.RegisterWorkers(
			workers,
			topicId,
			simulationData,
			workersPerTopic,
		)
		if err != nil {
			log.Fatal().Err(err).Msgf("Error registering workers: %v", err)
		}
		time.Sleep(20 * time.Second)
	}

	err = stress.FundTopics(
		faucet,
		topicIds,
	)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error funding topics: %v", err)
	}

	err = stress.StartActorLoops(
		simulationData,
		&config,
		topicIds,
	)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error starting actor loops: %v", err)
	}
}

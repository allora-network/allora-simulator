package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/allora-network/allora-simulator/types"
	"github.com/allora-network/allora-simulator/workloads/actors"
	"github.com/allora-network/allora-simulator/workloads/topics"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func main() {
	config := types.Config{}
	data, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}

	mnemonic, err := os.ReadFile("seedphrase")
	if err != nil {
		log.Fatalf("Failed to read seed phrase: %v", err)
	}

	// Set Bech32 prefixes and seal the configuration once
	sdkConfig := sdk.GetConfig()
	sdkConfig.SetBech32PrefixForAccount(config.Prefix, config.Prefix+"pub")
	sdkConfig.SetBech32PrefixForValidator(config.Prefix+"valoper", config.Prefix+"valoperpub")
	sdkConfig.SetBech32PrefixForConsensusNode(config.Prefix+"valcons", config.Prefix+"valconspub")
	sdkConfig.Seal()

	numActors := (config.WorkersPerTopic + config.ReputersPerTopic) * config.NumTopics
	faucet, simulationData := actors.CreateAndFundActors(
		&config,
		mnemonic,
		numActors,
		config.EpochLength,
	)

	// Create topics
	topicIds, err := topics.CreateTopics(
		faucet,
		config.NumTopics,
		config.EpochLength,
		config.CreateTopicsSameBlock,
	)
	if err != nil {
		log.Fatalf("Failed to create topics: %v", err)
	}

	// Calculate actors per topic
	actorsPerTopic := config.WorkersPerTopic + config.ReputersPerTopic

	// Register actors
	for i, topicId := range topicIds {
		// Get the slice of actors for this topic
		startIdx := i * actorsPerTopic
		topicActors := simulationData.Actors[startIdx : startIdx+actorsPerTopic]

		workers := topicActors[:config.WorkersPerTopic]
		reputers := topicActors[config.WorkersPerTopic:]

		time.Sleep(20 * time.Second)
		log.Printf("Registering reputers and adding stake in  topic: %d", topicId)
		err = actors.RegisterReputersAndStake(
			reputers,
			topicId,
			simulationData,
			config.ReputersPerTopic,
		)
		if err != nil {
			log.Fatalf("Error registering reputers: %v", err)
		}
		time.Sleep(20 * time.Second)
		log.Printf("Registering workers in  topic: %d", topicId)
		err = actors.RegisterWorkers(
			workers,
			topicId,
			simulationData,
			config.WorkersPerTopic,
		)
		if err != nil {
			log.Fatalf("Error registering workers: %v", err)
		}
		time.Sleep(20 * time.Second)
	}

	err = topics.FundTopics(
		faucet,
		topicIds,
	)
	if err != nil {
		log.Fatalf("Error funding topics: %v", err)
	}

	err = actors.StartActorLoops(
		simulationData,
		&config,
		topicIds,
		30,
	)
	if err != nil {
		log.Fatalf("Error starting actor loops: %v", err)
	}
}

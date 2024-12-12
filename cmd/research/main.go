package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/allora-network/allora-simulator/types"
	"github.com/allora-network/allora-simulator/workloads/research"
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

	mnemonic, err := os.ReadFile("scripts/seedphrase")
	if err != nil {
		log.Fatalf("Failed to read seed phrase: %v", err)
	}

	// Set Bech32 prefixes and seal the configuration once
	sdkConfig := sdk.GetConfig()
	sdkConfig.SetBech32PrefixForAccount(config.Prefix, config.Prefix+"pub")
	sdkConfig.SetBech32PrefixForValidator(config.Prefix+"valoper", config.Prefix+"valoperpub")
	sdkConfig.SetBech32PrefixForConsensusNode(config.Prefix+"valcons", config.Prefix+"valconspub")
	sdkConfig.Seal()

	// Calculate total number of actors needed
	totalActors := config.InferersPerTopic + config.ForecastersPerTopic + config.ReputersPerTopic
	faucet, simulationData := research.CreateAndFundActors(
		&config,
		mnemonic,
		totalActors,
		config.Research.Topic.EpochLength,
	)

	// Create research topic
	topicId, err := research.CreateAndFundResearchTopic(faucet, &config)
	if err != nil {
		log.Fatalf("Failed to create research topic: %v", err)
	}

	// Divide actors into their roles
	startIdx := 0
	inferers := simulationData.Actors[startIdx : startIdx+config.InferersPerTopic]
	startIdx += config.InferersPerTopic

	forecasters := simulationData.Actors[startIdx : startIdx+config.ForecastersPerTopic]
	startIdx += config.ForecastersPerTopic

	reputers := simulationData.Actors[startIdx : startIdx+config.ReputersPerTopic]

	// Register actors with delays between registrations
	time.Sleep(20 * time.Second)
	log.Printf("Registering reputers and adding stake for topic: %d", topicId)
	err = research.RegisterReputersAndStake(
		reputers,
		topicId,
		simulationData,
		config.ReputersPerTopic,
		true,
	)
	if err != nil {
		log.Fatalf("Error registering reputers: %v", err)
	}

	time.Sleep(20 * time.Second)
	log.Printf("Registering inferers for topic: %d", topicId)
	err = research.RegisterWorkers(
		inferers,
		topicId,
		simulationData,
		config.InferersPerTopic,
		true,
	)
	if err != nil {
		log.Fatalf("Error registering inferers: %v", err)
	}

	time.Sleep(20 * time.Second)
	log.Printf("Registering forecasters for topic: %d", topicId)
	err = research.RegisterWorkers(
		forecasters,
		topicId,
		simulationData,
		config.ForecastersPerTopic,
		false,
	)
	if err != nil {
		log.Fatalf("Error registering forecasters: %v", err)
	}

	// Start the simulation loops
	err = research.StartActorLoops(
		simulationData,
		&config,
		[]uint64{topicId},
	)
	if err != nil {
		log.Fatalf("Error starting actor loops: %v", err)
	}
}

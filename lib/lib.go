package lib

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	cosmosmath "cosmossdk.io/math"
	"github.com/allora-network/allora-simulator/client"
	"github.com/allora-network/allora-simulator/types"
)

func GetAccountInfo(address string, config *types.Config) (seqint, accnum uint64, err error) {
	resp, err := client.HTTPGet(config.Nodes.API + "/cosmos/auth/v1beta1/accounts/" + address)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get initial sequence: %v", err)
	}

	var accountRes types.AccountResult
	err = json.Unmarshal(resp, &accountRes)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to unmarshal account result: %v", err)
	}

	seqint, err = strconv.ParseUint(accountRes.Account.Sequence, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to convert sequence to int: %v", err)
	}

	accnum, err = strconv.ParseUint(accountRes.Account.AccountNumber, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to convert account number to int: %v", err)
	}

	return seqint, accnum, nil
}

func GetAccountBalance(address string, config *types.Config) (cosmosmath.Int, error) {
	resp, err := client.HTTPGet(config.Nodes.API + "/cosmos/bank/v1beta1/balances/" + address)
	if err != nil {
		return cosmosmath.ZeroInt(), err
	}

	var balanceRes types.BalanceResult
	err = json.Unmarshal(resp, &balanceRes)
	if err != nil {
		return cosmosmath.ZeroInt(), err
	}

	for _, coin := range balanceRes.Balances {
		if coin.Denom == config.Denom {
			amount, ok := cosmosmath.NewIntFromString(coin.Amount)
			if !ok {
				return cosmosmath.ZeroInt(), errors.New("invalid coin amount")
			}
			return amount, nil
		}
	}

	// If no balance found for the denom, return zero balance
	return cosmosmath.ZeroInt(), fmt.Errorf("denomination %s not found in account balances", config.Denom)
}

func GetNextTopicId(config *types.Config) (uint64, error) {
	resp, err := client.HTTPGet(config.Nodes.API + "/emissions/v5/next_topic_id")
	if err != nil {
		return 0, err
	}

	var topicIdRes types.NextTopicIdResult
	err = json.Unmarshal(resp, &topicIdRes)
	if err != nil {
		return 0, err
	}

	topicId, err := strconv.ParseUint(topicIdRes.TopicId, 10, 64)
	if err != nil {
		return 0, err
	}

	return topicId, nil
}

// Get the latest open worker nonce for a topic
func GetLatestOpenWorkerNonceByTopicId(config *types.Config, topicId uint64) (int64, error) {
	resp, err := client.HTTPGet(config.Nodes.API + "/emissions/v5/unfulfilled_worker_nonces/" + strconv.FormatUint(topicId, 10))
	if err != nil {
		return 0, err
	}

	var res types.UnfulfilledWorkerNoncesResult
	err = json.Unmarshal(resp, &res)
	if err != nil {
		return 0, err
	}

	if len(res.Nonces.Nonces) == 0 {
		return 0, err
	}

	// Convert to int64
	blockHeight, err := strconv.ParseInt(res.Nonces.Nonces[0].BlockHeight, 10, 64)
	if err != nil {
		return 0, err
	}

	return blockHeight, nil
}

// Get the oldest reputer nonce for a topic
func GetOldestReputerNonceByTopicId(config *types.Config, topicId uint64) (int64, error) {
	resp, err := client.HTTPGet(config.Nodes.API + "/emissions/v5/unfulfilled_reputer_nonces/" + strconv.FormatUint(topicId, 10))
	if err != nil {
		return 0, err
	}

	var res types.UnfulfilledReputerNoncesResult
	err = json.Unmarshal(resp, &res)
	if err != nil {
		return 0, err
	}

	if len(res.Nonces.Nonces) == 0 {
		return 0, nil
	}

	// Convert to int64
	blockHeight, err := strconv.ParseInt(res.Nonces.Nonces[len(res.Nonces.Nonces)-1].ReputerNonce.BlockHeight, 10, 64)
	if err != nil {
		return 0, err
	}

	return blockHeight, nil
}

// Get the active workers for a topic at a given block height to use for reputer payloads
func GetActiveWorkersForTopic(config *types.Config, topicId uint64, blockHeight int64) ([]string, error) {
	resp, err := client.HTTPGet(config.Nodes.API + "/emissions/v5/inferences/" + strconv.FormatUint(topicId, 10) + "/" + strconv.FormatInt(blockHeight, 10))
	if err != nil {
		return []string{}, err
	}

	var res types.InferencesAtBlockResult
	err = json.Unmarshal(resp, &res)
	if err != nil {
		return []string{}, err
	}

	workers := make([]string, 0)
	for _, inference := range res.Inferences.Inferences {
		workers = append(workers, inference.Inferer)
	}

	return workers, nil
}

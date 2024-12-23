package lib

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	cosmosmath "cosmossdk.io/math"
	alloramath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
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
	resp, err := client.HTTPGet(config.Nodes.API + "/emissions/v7/next_topic_id")
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
	resp, err := client.HTTPGet(config.Nodes.API + "/emissions/v7/unfulfilled_worker_nonces/" + strconv.FormatUint(topicId, 10))
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
	resp, err := client.HTTPGet(config.Nodes.API + "/emissions/v7/unfulfilled_reputer_nonces/" + strconv.FormatUint(topicId, 10))
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
	resp, err := client.HTTPGet(config.Nodes.API + "/emissions/v7/inferences/" + strconv.FormatUint(topicId, 10) + "/" + strconv.FormatInt(blockHeight, 10))
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

func GetNetworkInferencesAtBlock(config *types.Config, topicId uint64, blockHeight int64) (*emissionstypes.ValueBundle, error) {
	resp, err := client.HTTPGet(fmt.Sprintf("%s/emissions/v7/network_inferences/%d/last_inference/%d",
		config.Nodes.API,
		topicId,
		blockHeight))
	if err != nil {
		return nil, fmt.Errorf("failed to get reputer values: %v", err)
	}

	var networkInferencesRes *types.GetNetworkInferencesAtBlockResponse
	err = json.Unmarshal(resp, &networkInferencesRes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal network inferences result: %v", err)
	}

	// Convert from API version to Proto version
	combinedValue, err := alloramath.NewDecFromString(networkInferencesRes.NetworkInferences.CombinedValue)
	if err != nil {
		return nil, fmt.Errorf("invalid combined value: %w", err)
	}
	naiveValue, err := alloramath.NewDecFromString(networkInferencesRes.NetworkInferences.NaiveValue)
	if err != nil {
		return nil, fmt.Errorf("invalid naive value: %w", err)
	}

	infererValues := make([]*emissionstypes.WorkerAttributedValue, len(networkInferencesRes.NetworkInferences.InfererValues))
	for i, v := range networkInferencesRes.NetworkInferences.InfererValues {
		value, err := alloramath.NewDecFromString(v.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid inferer value at index %d: %w", i, err)
		}
		infererValues[i] = &emissionstypes.WorkerAttributedValue{
			Worker: v.Worker,
			Value:  value,
		}
	}

	forecasterValues := make([]*emissionstypes.WorkerAttributedValue, len(networkInferencesRes.NetworkInferences.ForecasterValues))
	for i, v := range networkInferencesRes.NetworkInferences.ForecasterValues {
		value, err := alloramath.NewDecFromString(v.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid forecaster value at index %d: %w", i, err)
		}
		forecasterValues[i] = &emissionstypes.WorkerAttributedValue{
			Worker: v.Worker,
			Value:  value,
		}
	}

	oneOutInfererValues := make([]*emissionstypes.WithheldWorkerAttributedValue, len(networkInferencesRes.NetworkInferences.OneOutInfererValues))
	for i, v := range networkInferencesRes.NetworkInferences.OneOutInfererValues {
		value, err := alloramath.NewDecFromString(v.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid one out inferer value at index %d: %w", i, err)
		}
		oneOutInfererValues[i] = &emissionstypes.WithheldWorkerAttributedValue{
			Worker: v.Worker,
			Value:  value,
		}
	}

	oneOutForecasterValues := make([]*emissionstypes.WithheldWorkerAttributedValue, len(networkInferencesRes.NetworkInferences.OneOutForecasterValues))
	for i, v := range networkInferencesRes.NetworkInferences.OneOutForecasterValues {
		value, err := alloramath.NewDecFromString(v.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid one out forecaster value at index %d: %w", i, err)
		}
		oneOutForecasterValues[i] = &emissionstypes.WithheldWorkerAttributedValue{
			Worker: v.Worker,
			Value:  value,
		}
	}

	oneInForecasterValues := make([]*emissionstypes.WorkerAttributedValue, len(networkInferencesRes.NetworkInferences.OneInForecasterValues))
	for i, v := range networkInferencesRes.NetworkInferences.OneInForecasterValues {
		value, err := alloramath.NewDecFromString(v.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid one in forecaster value at index %d: %w", i, err)
		}
		oneInForecasterValues[i] = &emissionstypes.WorkerAttributedValue{
			Worker: v.Worker,
			Value:  value,
		}
	}

	oneOutInfererForecasterValues := make([]*emissionstypes.OneOutInfererForecasterValues, len(networkInferencesRes.NetworkInferences.OneOutInfererForecasterValues))
	for i, v := range networkInferencesRes.NetworkInferences.OneOutInfererForecasterValues {
		oneOutInfererValues := make([]*emissionstypes.WithheldWorkerAttributedValue, len(v.OneOutInfererValues))
		for j, ov := range v.OneOutInfererValues {
			value, err := alloramath.NewDecFromString(ov.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid one out inferer forecaster value at index %d,%d: %w", i, j, err)
			}
			oneOutInfererValues[j] = &emissionstypes.WithheldWorkerAttributedValue{
				Worker: ov.Worker,
				Value:  value,
			}
		}
		oneOutInfererForecasterValues[i] = &emissionstypes.OneOutInfererForecasterValues{
			Forecaster:          v.Forecaster,
			OneOutInfererValues: oneOutInfererValues,
		}
	}

	return &emissionstypes.ValueBundle{
		CombinedValue:                 combinedValue,
		NaiveValue:                    naiveValue,
		InfererValues:                 infererValues,
		ForecasterValues:              forecasterValues,
		OneOutInfererValues:           oneOutInfererValues,
		OneOutForecasterValues:        oneOutForecasterValues,
		OneInForecasterValues:         oneInForecasterValues,
		OneOutInfererForecasterValues: oneOutInfererForecasterValues,
	}, nil
}

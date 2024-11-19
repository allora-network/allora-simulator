package transaction

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/allora-network/allora-simulator/client"
	"github.com/allora-network/allora-simulator/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
)

var cdc = codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

// Loop handles the main transaction broadcasting logic
func SendDataWithRetry(
	txParams *types.TransactionParams,
	msgs ...sdktypes.Msg,
) (*coretypes.ResultBroadcastTx, uint64, error) {
	sequence := txParams.Sequence
	maxRetries := int64(5)
	retryDelay := int64(4)

	for retryCount := int64(0); retryCount <= maxRetries; retryCount++ {
		currentSequence := sequence

		resp, _, err := SendTransactionViaRPC(txParams, currentSequence, msgs...)
		if err != nil {
			fmt.Printf("Error broadcasting transaction: %v\n", err)
			// if sequence mismatch, handle it and retry
			if resp != nil && resp.Code == 32 {
				resp, newSeq, err := handleSequenceMismatch(txParams, sequence, err, msgs...)
				if err == nil {
					sequence = newSeq
					return resp, sequence, nil
				}
				continue
			}
			// if mempool is full, retry
			if strings.Contains(err.Error(), "mempool is full") {
				delay := calculateLinearBackoffDelay(retryDelay, retryCount+1)
				fmt.Printf("Mempool is full, retrying in %d seconds...\n", delay/time.Second)
				time.Sleep(delay)
				continue
			}
			continue
		}

		sequence++
		return resp, sequence, nil
	}

	return nil, sequence, nil
}

// SendTransactionViaRPC sends a transaction using the provided TransactionParams and sequence number.
func SendTransactionViaRPC(txParams *types.TransactionParams, sequence uint64, msgs ...sdktypes.Msg) (*coretypes.ResultBroadcastTx, string, error) {
	encodingConfig := moduletestutil.MakeTestEncodingConfig()
	encodingConfig.Codec = cdc

	ctx := context.Background()

	// Build and sign the transaction
	txBytes, err := BuildAndSignTransaction(ctx, txParams, sequence, encodingConfig, msgs...)
	if err != nil {
		return nil, "", err
	}

	// Broadcast the transaction via RPC
	resp, err := Transaction(txBytes, txParams.NodeURL)
	if err != nil {
		return resp, string(txBytes), fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	return resp, string(txBytes), nil
}

// Transaction broadcasts the transaction bytes to the given RPC endpoint.
func Transaction(txBytes []byte, rpcEndpoint string) (*coretypes.ResultBroadcastTx, error) {
	client, err := client.GetClient(rpcEndpoint)
	if err != nil {
		return nil, err
	}

	return client.BroadcastTx(txBytes)
}

// handleSequenceMismatch handles the case where a transaction fails due to sequence mismatch
func handleSequenceMismatch(txParams *types.TransactionParams, sequence uint64, err error, msgs ...sdktypes.Msg) (*coretypes.ResultBroadcastTx, uint64, error) {
	expectedSeq, parseErr := extractExpectedSequence(err.Error())
	if parseErr != nil {
		fmt.Printf("Failed to parse expected sequence: %v\n", parseErr)
		return nil, sequence, nil
	}

	resp, _, err := SendTransactionViaRPC(txParams, expectedSeq, msgs...)
	if err != nil {
		return nil, expectedSeq, err
	}

	return resp, expectedSeq + 1, nil
}

// Function to extract the expected sequence number from the error message
func extractExpectedSequence(errMsg string) (uint64, error) {
	// Parse the error message to extract the expected sequence number
	if !strings.Contains(errMsg, "account sequence mismatch") {
		return 0, fmt.Errorf("unexpected error message format: %s", errMsg)
	}

	index := strings.Index(errMsg, "expected ")
	if index == -1 {
		return 0, errors.New("expected sequence not found in error message")
	}

	start := index + len("expected ")
	rest := errMsg[start:]
	parts := strings.SplitN(rest, ",", 2)
	if len(parts) < 1 {
		return 0, errors.New("failed to split expected sequence from error message")
	}

	expectedSeqStr := strings.TrimSpace(parts[0])
	expectedSeq, err := strconv.ParseUint(expectedSeqStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse expected sequence number: %v", err)
	}

	return expectedSeq, nil
}

func calculateLinearBackoffDelay(baseDelay int64, retryCount int64) time.Duration {
	return time.Duration(baseDelay*retryCount) * time.Second
}

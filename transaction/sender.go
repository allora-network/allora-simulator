package transaction

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	waitForTx bool,
	msgs ...sdktypes.Msg,
) (*coretypes.ResultBroadcastTx, uint64, error) {
	sequence := txParams.Sequence
	maxRetries := int64(5)
	retryDelay := int64(4)

	for retryCount := int64(0); retryCount <= maxRetries; retryCount++ {
		currentSequence := sequence

		resp, _, err := SendTransactionViaRPC(txParams, currentSequence, waitForTx, msgs...)
		if err != nil {
			log.Printf("Transaction failed: %v\n", err)
			log.Printf("Handling error and retrying...")

			// if sequence mismatch, handle it and retry
			if strings.Contains(err.Error(), "account sequence mismatch") {
				resp, newSeq, err := handleSequenceMismatch(txParams, sequence, waitForTx, err, msgs...)
				if err == nil {
					sequence = newSeq
					return resp, sequence, nil
				}
				continue
			}
			// if mempool is full, retry
			if strings.Contains(err.Error(), "mempool is full") {
				delay := calculateLinearBackoffDelay(retryDelay, retryCount+1)
				time.Sleep(delay)
				continue
			}
			// connection issues
			if strings.Contains(err.Error(), "connection reset by peer") {
				delay := calculateLinearBackoffDelay(retryDelay, retryCount+1)
				time.Sleep(delay)
				continue
			}
			// print if other errors
			log.Printf("Error: %v\n", err)
			continue
		}
		if resp != nil {
			log.Printf("Transaction sent successfully: %v\n", resp.Hash.String())
		}
	
		sequence++
		return resp, sequence, nil
	}

	return nil, sequence, nil
}

// SendTransactionViaRPC sends a transaction using the provided TransactionParams and sequence number.
func SendTransactionViaRPC(txParams *types.TransactionParams, sequence uint64, waitForTx bool, msgs ...sdktypes.Msg) (*coretypes.ResultBroadcastTx, string, error) {
	encodingConfig := moduletestutil.MakeTestEncodingConfig()
	encodingConfig.Codec = cdc

	ctx := context.Background()

	// Build and sign the transaction
	txBytes, err := BuildAndSignTransaction(ctx, txParams, sequence, encodingConfig, msgs...)
	if err != nil {
		return nil, "", err
	}

	// Broadcast the transaction via RPC
	resp, err := Transaction(txBytes, txParams.Config.Nodes.RPC[0], waitForTx)
	if err != nil {
		return resp, string(txBytes), fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	return resp, string(txBytes), nil
}

// Transaction broadcasts the transaction bytes to the given RPC endpoint.
func Transaction(txBytes []byte, rpcEndpoint string, waitForTx bool) (*coretypes.ResultBroadcastTx, error) {
	client, err := client.GetClient(rpcEndpoint)
	if err != nil {
		return nil, err
	}
	resp, err := client.BroadcastTx(txBytes, waitForTx)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	return resp, nil
}

// handleSequenceMismatch handles the case where a transaction fails due to sequence mismatch
func handleSequenceMismatch(txParams *types.TransactionParams, sequence uint64, waitForTx bool, err error, msgs ...sdktypes.Msg) (*coretypes.ResultBroadcastTx, uint64, error) {
	expectedSeq, parseErr := extractExpectedSequence(err.Error())
	if parseErr != nil {
		return nil, sequence, nil
	}

	resp, _, err := SendTransactionViaRPC(txParams, expectedSeq, waitForTx, msgs...)
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

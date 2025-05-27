package common

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/allora-network/allora-simulator/client"
	"github.com/allora-network/allora-simulator/lib"
	"github.com/allora-network/allora-simulator/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/rs/zerolog/log"
)

var cdc = codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

func BuildAndSignTransaction(
	ctx context.Context,
	txParams *types.TransactionParams,
	sequence uint64,
	encodingConfig moduletestutil.TestEncodingConfig,
	msgs ...sdktypes.Msg,
) ([]byte, error) {
	// Create a new TxBuilder
	txBuilder := encodingConfig.TxConfig.NewTxBuilder()

	var memo string

	// Construct the message based on the message type
	var err error

	// Set the message and other transaction parameters
	if err := txBuilder.SetMsgs(msgs...); err != nil {
		return nil, err
	}

	// Estimate gas limit
	totalTxSize := 0
	for _, msg := range msgs {
		totalTxSize += len(msg.String())
	}
	gas, err := EstimateGas(totalTxSize, txParams.Config)
	if err != nil {
		return nil, err
	}
	// Apply adjustment safely
	if txParams.Config.GasAdjustment > 0 {
		gasFloat := float64(gas) * txParams.Config.GasAdjustment
	if gasFloat < math.MaxUint64 {
		gas = uint64(gasFloat)
	} else {
			gas = math.MaxUint64
		}
	}
	txBuilder.SetGasLimit(gas)

	// Calculate fee
	minGasPrice := lib.GetCurrentGasPrice()
	fees, err := CalculateFees(gas, minGasPrice)
	if err != nil {
		return nil, err
	}
	feeCoin := sdktypes.NewCoin(txParams.Config.Denom, fees)
	txBuilder.SetFeeAmount(sdktypes.NewCoins(feeCoin))

	// Set memo and timeout height
	txBuilder.SetMemo(memo)
	txBuilder.SetTimeoutHeight(0)

	// Set up signature
	sigV2 := signing.SignatureV2{
		PubKey:   txParams.PubKey,
		Sequence: sequence,
		Data: &signing.SingleSignatureData{
			SignMode: signing.SignMode_SIGN_MODE_DIRECT,
		},
	}

	if err := txBuilder.SetSignatures(sigV2); err != nil {
		return nil, err
	}

	signerData := authsigning.SignerData{
		ChainID:       txParams.Config.ChainID,
		AccountNumber: txParams.AccNum,
		Sequence:      sequence,
	}

	// Sign the transaction with the private key
	sigV2, err = tx.SignWithPrivKey(
		ctx,
		signing.SignMode_SIGN_MODE_DIRECT,
		signerData,
		txBuilder,
		txParams.PrivKey,
		encodingConfig.TxConfig,
		sequence,
	)
	if err != nil {
		return nil, err
	}

	// Set the signed signature back to the txBuilder
	if err := txBuilder.SetSignatures(sigV2); err != nil {
		return nil, err
	}

	// Encode the transaction
	txBytes, err := encodingConfig.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, err
	}

	return txBytes, nil
}

// Loop handles the main transaction broadcasting logic
func SendDataWithRetry(
	txParams *types.TransactionParams,
	waitForTx bool,
	msgs ...sdktypes.Msg,
) (*coretypes.ResultBroadcastTx, uint64, error) {
	sequence := txParams.Sequence

	for retryCount := int64(0); retryCount <= maxRetries; retryCount++ {
		currentSequence := sequence

		resp, _, err := sendTransactionViaRPC(txParams, currentSequence, waitForTx, msgs...)
		if err != nil {
			log.Error().Msgf("Transaction failed: %v", err)
			log.Info().Msgf("Handling error and retrying...")

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
			log.Error().Msgf("Error: %v", err)
			continue
		}
		if resp != nil {
			if resp.Code != 0 {
				log.Error().Msgf("Error on the broadcasted transaction: %v", resp.Log)
				delay := calculateLinearBackoffDelay(retryDelay, retryCount+1)
				time.Sleep(delay)
				continue
			} else {
				log.Info().Msgf("Transaction sent successfully: %v", resp.Hash.String())
			}
		}

		sequence++
		return resp, sequence, nil
	}

	return nil, sequence, nil
}

// sendTransactionViaRPC sends a transaction using the provided TransactionParams and sequence number.
func sendTransactionViaRPC(txParams *types.TransactionParams, sequence uint64, waitForTx bool, msgs ...sdktypes.Msg) (*coretypes.ResultBroadcastTx, string, error) {
	encodingConfig := moduletestutil.MakeTestEncodingConfig()
	encodingConfig.Codec = cdc

	ctx := context.Background()

	// Build and sign the transaction
	txBytes, err := BuildAndSignTransaction(ctx, txParams, sequence, encodingConfig, msgs...)
	if err != nil {
		return nil, "", err
	}

	// Broadcast the transaction via RPC
	resp, err := broadcastTransaction(txBytes, txParams.Config.Nodes.RPC[0], waitForTx)
	if err != nil {
		return resp, string(txBytes), fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	return resp, string(txBytes), nil
}

// broadcastTransaction broadcasts the transaction bytes to the given RPC endpoint.
func broadcastTransaction(txBytes []byte, rpcEndpoint string, waitForTx bool) (*coretypes.ResultBroadcastTx, error) {
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

	resp, _, err := sendTransactionViaRPC(txParams, expectedSeq, waitForTx, msgs...)
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

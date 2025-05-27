package common

import (
	"fmt"
	"math"
	"time"

	"github.com/allora-network/allora-simulator/lib"
	types "github.com/allora-network/allora-simulator/types"

	cosmossdk_io_math "cosmossdk.io/math"
	"github.com/rs/zerolog/log"
)

const (
	maxRetries = 5
	retryDelay = 4
)

// EstimateGas calculates the estimated gas for a transaction based on its size.
func EstimateGas(txSize int, config *types.Config) (uint64, error) {
	if txSize < 0 {
		return 0, fmt.Errorf("transaction size cannot be negative")
	}

	// Calculate gas for transaction size
	sizeGas := uint64(txSize) * config.GasPerByte

	// Total gas is base gas + size gas
	totalGas := config.BaseGas + sizeGas
	if totalGas < config.BaseGas {
		return 0, fmt.Errorf("total gas overflows")
	}
	return totalGas, nil
}

// CalculateFees safely computes the fee amount.
func CalculateFees(gas uint64, minGasPrice float64) (cosmossdk_io_math.Int, error) {
	if gas == 0 {
		return cosmossdk_io_math.NewInt(0), fmt.Errorf("gas cannot be zero")
	}
	if minGasPrice <= 0 {
		return cosmossdk_io_math.NewInt(0), fmt.Errorf("minimum gas price must be greater than zero")
	}

	// Convert gas and gas price to fee with rounding
	floatFee := math.Round(float64(gas) * minGasPrice)
	if floatFee > math.MaxUint64 {
		return cosmossdk_io_math.NewInt(0), fmt.Errorf("fee overflows")
	}

	// Convert to uint safely
	uintFee := uint64(floatFee)
	fee := cosmossdk_io_math.NewIntFromUint64(uintFee)

	return fee, nil
}

// Update the gas price periodically
func RunGasRoutine(
	config *types.Config,
) {
	for {
		gasPrice, err := lib.GetGasPrice(config)
		if err != nil {
			log.Error().Err(err).Msgf("Error getting base fee, will retry: %v", err)
			// Continue to the sleep and try again
		} else {
			lib.SetCurrentGasPrice(gasPrice)
		}
		time.Sleep(2 * time.Second)
	}
}

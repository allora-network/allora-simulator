package lib

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/allora-network/allora-simulator/client"
	"github.com/allora-network/allora-simulator/types"
	feemarkettypes "github.com/skip-mev/feemarket/x/feemarket/types"
)

// Keeps track of the current gas price
var gasPrice float64 = 0

// GetCurrentGasPrice returns the current gas price
func GetCurrentGasPrice() float64 {
	return gasPrice
}

// SetCurrentGasPrice sets the current gas price
func SetCurrentGasPrice(price float64) {
	gasPrice = price
}

// GetGasPrice queries the current gas price from the feemarket module
func GetGasPrice(config *types.Config) (float64, error) {
	resp, err := client.HTTPGet(config.Nodes.API + "/feemarket/v1/gas_price/uallo")
	if err != nil {
		return 0, err
	}
	var gasPriceRes *feemarkettypes.GasPriceResponse
	err = json.Unmarshal(resp, &gasPriceRes)
	if err != nil {
		return 0, fmt.Errorf("failed to unmarshal gas price result: %w", err)
	}

	// Convert legacyDec to string first, then to float64
	gasPriceStr := gasPriceRes.Price.Amount.String()
	gasPrice, err := strconv.ParseFloat(gasPriceStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse gas price: %w", err)
	}

	return gasPrice, nil
}

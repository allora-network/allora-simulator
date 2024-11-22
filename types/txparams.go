package types

import (
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

type TransactionParams struct {
	Config   *Config
	Sequence uint64
	AccNum   uint64
	PrivKey  cryptotypes.PrivKey
	PubKey   cryptotypes.PubKey
}

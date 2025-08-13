package common

import (
	"fmt"
	"io"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

func GeneratePrivKey(rand io.Reader) (cryptotypes.PrivKey, cryptotypes.PubKey, string) {
	privKey, err := secp256k1.GeneratePrivateKeyFromRand(rand)
	if err != nil {
		panic(err)
	}

	algo := hd.Secp256k1
	privKeyCrypto := algo.Generate()(privKey.Serialize())
	pubKey := privKeyCrypto.PubKey()

	addressbytes := sdk.AccAddress(pubKey.Address().Bytes())
	address, err := sdk.Bech32ifyAddressBytes("allo", addressbytes)
	if err != nil {
		panic(err)
	}

	return privKeyCrypto, pubKey, address
}

func GetPrivKey(prefix string, mnemonic []byte) (cryptotypes.PrivKey, cryptotypes.PubKey, string) {
	algo := hd.Secp256k1

	hdPath := fmt.Sprintf("m/44'/%d'/0'/0/%d", 118, 0)
	derivedPriv, err := algo.Derive()(string(mnemonic), "", hdPath)
	if err != nil {
		panic(err)
	}

	privKey := algo.Generate()(derivedPriv)
	pubKey := privKey.PubKey()

	addressbytes := sdk.AccAddress(pubKey.Address().Bytes())
	address, err := sdk.Bech32ifyAddressBytes(prefix, addressbytes)
	if err != nil {
		panic(err)
	}

	return privKey, pubKey, address
}

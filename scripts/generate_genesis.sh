#!/usr/bin/env bash

# This script generates a genesis file for use with localnet and integration tests
# This is just a sample of how you could make a genesis file for your own allorad network.

echo "Generate Genesis"

set -eu

APP_HOME="${APP_HOME:-$(pwd)}"
SEEDPHRASE="${SEEDPHRASE:-/scripts/seedphrase}"
CHAIN_ID="localnet"
VALIDATOR_NUMBER=${VALIDATOR_NUMBER:-3}
valPreffix="validator"
DENOM="uallo"
KEYRING_BACKEND=test

allorad=$(which allorad)
echo "Using allorad binary at $allorad"

genesisHome="$APP_HOME/genesis"
gentxDir=${genesisHome}/gentxs

echo "Starting genesis generation for chain $CHAIN_ID with $VALIDATOR_NUMBER validators"
mkdir -p $gentxDir

$allorad --home=$genesisHome init mymoniker --chain-id $CHAIN_ID --default-denom ${DENOM}

#Create validators account
for ((i=0; i<$VALIDATOR_NUMBER; i++)); do
    valName="${valPreffix}${i}"

    echo "Generate $valName account"
    $allorad --home=$genesisHome keys add $valName \
        --keyring-backend $KEYRING_BACKEND > "$APP_HOME/$valName.account_info" 2>&1

    echo "Fund $valName account to genesis, VALIDATOR_TOKENS: 10000000allo "
    echo "$allorad --home=$genesisHome genesis add-genesis-account $valName 10000000allo --keyring-backend $KEYRING_BACKEND"
    $allorad --home=$genesisHome genesis add-genesis-account \
        $valName 10000000allo \
        --keyring-backend $KEYRING_BACKEND

done

FAUCET_ACCOUNT_INFO_FILE="$APP_HOME/faucet.account_info"

echo "Generating faucet account and saving seed phrase (full info to $FAUCET_ACCOUNT_INFO_FILE)..."
$allorad --home $genesisHome keys add faucet --keyring-backend $KEYRING_BACKEND 2>&1 | tee $APP_HOME/faucet.account_info | tail -n 1 | tr -d '\n' | tee "$APP_HOME/seedphrase" "$SEEDPHRASE" > /dev/null

# Display the seed phrase if the script is run interactively
if [ -t 1 ]; then # Check if stdout is a terminal
    if [ -f "$APP_HOME/seedphrase" ]; then
        FAUCET_SEED_PHRASE_FOR_DISPLAY=$(cat "$APP_HOME/seedphrase")
        echo "Extracted Faucet Seed Phrase (to $APP_HOME/seedphrase): $FAUCET_SEED_PHRASE_FOR_DISPLAY"
    elif [ -f "$SEEDPHRASE" ] && [ "$SEEDPHRASE" != "$APP_HOME/seedphrase" ]; then # Check the other location if different
        FAUCET_SEED_PHRASE_FOR_DISPLAY=$(cat "$SEEDPHRASE")
        echo "Extracted Faucet Seed Phrase (to $SEEDPHRASE): $FAUCET_SEED_PHRASE_FOR_DISPLAY"
    fi
fi
echo "Full faucet account info originally from: $FAUCET_ACCOUNT_INFO_FILE"

echo "Fund faucet account"
$allorad --home=$genesisHome genesis add-genesis-account \
    faucet 10000000allo \
    --keyring-backend $KEYRING_BACKEND

for ((i=0; i<$VALIDATOR_NUMBER; i++)); do
    echo "Initializing Validator $i"

    valName="${valPreffix}${i}"
    valHome="$APP_HOME/data/$valName"
    mkdir -p $valHome

    $allorad --home=$valHome init $valName --chain-id $CHAIN_ID --default-denom ${DENOM}

    # Symlink genesis to have the accounts
    ln -sf $genesisHome/config/genesis.json $valHome/config/genesis.json

    # Symlink keyring-test to have keys
    ln -sf $genesisHome/keyring-test $valHome/keyring-test

    echo "Applying node-specific configurations for validator $valName..."
    # Configure config.toml for this validator
    dasel put -t string -v "kv" 'tx_index.indexer' -f $valHome/config/config.toml
    dasel put -t string -v "*:error,p2p:info,state:info,x/feemarket:error" 'log_level' -f $valHome/config/config.toml
    dasel put mempool.max_txs_bytes -t int -v 33554432 -f $valHome/config/config.toml
    dasel put mempool.size -t int -v 5000 -f $valHome/config/config.toml
    
    # Configure app.toml for this validator
    dasel put telemetry.enabled -t bool -v true -f $valHome/config/app.toml
    dasel put -t bool -v true 'api.enable' -f $valHome/config/app.toml
    dasel put -t bool -v true 'api.swagger' -f $valHome/config/app.toml
    dasel put -t bool -v true 'grpc.enable' -f $valHome/config/app.toml

    if [ "$i" -eq 0 ]; then
      echo "Applying specific address configurations for the first validator ($valName)..."
      dasel put -t string -v "tcp://0.0.0.0:26657" 'rpc.laddr' -f $valHome/config/config.toml
      dasel put -t string -v "0.0.0.0:1317" 'api.address' -f $valHome/config/app.toml
      dasel put -t string -v "0.0.0.0:9090" 'grpc.address' -f $valHome/config/app.toml
    fi

    echo "Generating gentx for $valName with 1000allo stake at $gentxDir/$valName.json"
    $allorad --home=$valHome genesis gentx $valName 1000allo \
        --chain-id $CHAIN_ID --keyring-backend $KEYRING_BACKEND \
        --moniker="$valName" \
        --from=$valName \
        --output-document $gentxDir/$valName.json
done

$allorad --home=$genesisHome genesis collect-gentxs --gentx-dir $gentxDir

#Set additional genesis params
echo "Get faucet address"
FAUCET_ADDRESS=$($allorad --home=$genesisHome keys show faucet -a --keyring-backend $KEYRING_BACKEND)
FAUCET_ADDRESS="${FAUCET_ADDRESS%%[[:cntrl:]]}"

# some sample default parameters for integration tests
dasel put 'app_state.emissions.core_team_addresses.append()' -t string -v $FAUCET_ADDRESS -f $genesisHome/config/genesis.json
dasel put 'app_state.gov.params.expedited_voting_period' -t string -v "300s" -f $genesisHome/config/genesis.json
dasel put 'app_state.feemarket.params.fee_denom' -t string -v "uallo" -f $genesisHome/config/genesis.json
# Additional genesis params from local_node.sh
dasel put 'app_state.feemarket.params.distribute_fees' -t bool -v true -f $genesisHome/config/genesis.json
dasel put 'app_state.emissions.params.global_whitelist_enabled' -t bool -v false -f $genesisHome/config/genesis.json
dasel put 'app_state.emissions.params.topic_creator_whitelist_enabled' -t bool -v false -f $genesisHome/config/genesis.json
dasel put -t string -v "$FAUCET_ADDRESS" 'app_state.emissions.whitelist_admins.append()' -f $genesisHome/config/genesis.json

cp -f $genesisHome/config/genesis.json "$APP_HOME"

echo "$CHAIN_ID genesis generated."

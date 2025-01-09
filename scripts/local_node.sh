#!/bin/bash
set -e

export APP_HOME="${APP_HOME:-./data}"
INIT_FLAG="${APP_HOME}/.initialized"
MONIKER="${MONIKER:-validator}"
KEYRING_BACKEND=test
CHAIN_ID="${CHAIN_ID:-localnet}"
DENOM="uallo"
RESET_DATA="${RESET_DATA:-false}"

BINARY=allorad

# Check if data should be reset
if [ -d "${APP_HOME}" ] && [ "$RESET_DATA" = "true" ]; then
    echo "Removing existing data..."
    # Remove contents of the directory instead of the directory itself
    rm -rf ${APP_HOME}/* ${APP_HOME}/.* 2>/dev/null || true
    rm -f $INIT_FLAG
fi

echo "To re-initiate the node, remove the file: ${INIT_FLAG}"
if [ ! -f $INIT_FLAG ]; then
    # Remove if existing config
    rm -rf ${APP_HOME}/config

    # Create symlink for allorad config
    ln -sf ${APP_HOME} ${HOME}/.allorad

    # Init node
    $BINARY --home=${APP_HOME} init ${MONIKER} --chain-id=${CHAIN_ID} --default-denom $DENOM

    # Create accounts
    $BINARY --home $APP_HOME keys add ${MONIKER} --keyring-backend $KEYRING_BACKEND > $APP_HOME/${MONIKER}.account_info 2>&1
    $BINARY --home $APP_HOME keys add faucet --keyring-backend $KEYRING_BACKEND 2>&1 | tee $APP_HOME/faucet.account_info | tail -n 1 | tr -d '\n' | tee $APP_HOME/seedphrase /scripts/seedphrase > /dev/null

    # Add genesis accounts
    $BINARY --home=${APP_HOME} genesis add-genesis-account $MONIKER 10000000allo --keyring-backend test
    $BINARY --home=${APP_HOME} genesis add-genesis-account faucet 10000000allo --keyring-backend test

    # Create validator transaction
    $BINARY --home=${APP_HOME} genesis gentx $MONIKER 1000allo --chain-id $CHAIN_ID --keyring-backend test

    # Collect genesis transactions
    $BINARY --home=${APP_HOME} genesis collect-gentxs

    # Setup allorad client
    $BINARY --home=${APP_HOME} config set client chain-id ${CHAIN_ID}
    $BINARY --home=${APP_HOME} config set client keyring-backend $KEYRING_BACKEND

    # Enable indexer
    dasel put -t string -v "kv" 'tx_index.indexer' -f ${APP_HOME}/config/config.toml

    # Configure mempool
    dasel put mempool.max_txs_bytes -t int -v 2097152 -f ${APP_HOME}/config/config.toml
    dasel put mempool.size -t int -v 1000 -f ${APP_HOME}/config/config.toml

    # Enable telemetry
    dasel put telemetry.enabled -t bool -v true -f ${APP_HOME}/config/app.toml

    # Configure API and gRPC
    dasel put -t bool -v true 'api.enable' -f ${APP_HOME}/config/app.toml
    dasel put -t bool -v true 'api.swagger' -f ${APP_HOME}/config/app.toml
    dasel put -t string -v "0.0.0.0:1317" 'api.address' -f ${APP_HOME}/config/app.toml
    dasel put -t bool -v true 'grpc.enable' -f ${APP_HOME}/config/app.toml
    dasel put -t string -v "0.0.0.0:9090" 'grpc.address' -f ${APP_HOME}/config/app.toml

    # Configure RPC
    dasel put -t string -v "tcp://0.0.0.0:26657" 'rpc.laddr' -f ${APP_HOME}/config/config.toml

    # Configure genesis parameters
    dasel put 'app_state.feemarket.params.fee_denom' -t string -v "uallo" -f ${APP_HOME}/config/genesis.json
    dasel put 'app_state.feemarket.params.distribute_fees' -t bool -v true -f ${APP_HOME}/config/genesis.json
    dasel put 'app_state.emissions.params.global_whitelist_enabled' -t bool -v false -f ${APP_HOME}/config/genesis.json
    dasel put 'app_state.emissions.params.topic_creator_whitelist_enabled' -t bool -v false -f ${APP_HOME}/config/genesis.json

    # Add faucet address to whitelist
    FAUCET_ADDRESS=$($BINARY --home $APP_HOME keys show faucet -a --keyring-backend $KEYRING_BACKEND)
    dasel put -t string -v "$FAUCET_ADDRESS" 'app_state.emissions.whitelist_admins.append()' -f ${APP_HOME}/config/genesis.json

    touch $INIT_FLAG
fi
echo "Node is initialized"

# Start the chain
echo "Starting validator node without cosmovisor"
allorad \
    --home=${APP_HOME} \
    start \
    --api.enable \
    --api.address=tcp://0.0.0.0:1317 \
    --moniker=${MONIKER} \
    --minimum-gas-prices=0${DENOM} \
    --rpc.laddr=tcp://0.0.0.0:26657
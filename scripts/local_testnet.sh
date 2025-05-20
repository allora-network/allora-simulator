#!/bin/bash
set -eu  #e

# Ensure we're in integration folder
cd "$(dirname "$0")"

# Function to check if a command exists
check_command() {
  if ! command -v "$1" &> /dev/null; then
    echo "Error: $1 is not installed."
    exit 1
  fi
}

# List of required commands
commands=("docker" "jq" "envsubst" "curl")

# Check each command
for cmd in "${commands[@]}"; do
  check_command "$cmd"
done

DOCKER_IMAGE="alloranetwork/allora-chain:v0.11.0"
VALIDATOR_NUMBER="${VALIDATOR_NUMBER:-3}"
VALIDATOR_PREFIX=validator
NETWORK_PREFIX="192.168.250"
VALIDATORS_IP_START=10
VALIDATORS_RPC_PORT_START=26657
VALIDATORS_API_PORT_START=1317
VALIDATORS_GRPC_PORT_START=9090
HEADS_IP_START=20
CHAIN_ID="${CHAIN_ID:-localnet}"
APP_HOME="$(pwd)/$CHAIN_ID"

ACCOUNTS_TOKENS=1000000

# Indexer and Producer Settings
INDEXER="${INDEXER:-false}"
PRODUCER="${PRODUCER:-false}"
PRODUCER_SEED_PHRASE="${PRODUCER_SEED_PHRASE:-}" # Mandatory if PRODUCER=true. Was ALLORA_PRODUCER_SEED_PHRASE_L1

# Docker Image References (as used in templates)
POSTGRES_IMAGE_REF="${POSTGRES_IMAGE_REF:-postgres:16-bookworm}"
INDEXER_IMAGE_REF="${INDEXER_IMAGE_REF:-alloranetwork/allora-indexer:v0.11.1}"
PRODUCER_IMAGE_REF="${PRODUCER_IMAGE_REF:-696230526504.dkr.ecr.us-east-1.amazonaws.com/allora-producer:v0.2.1}"

# Database Settings (as used in templates)
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-password}"
POSTGRES_DB_NAME="${POSTGRES_DB_NAME:-alloradb_l1}"
DB_HOST_PORT="${DB_HOST_PORT:-5432}" # Host port for DB
echo "DB_HOST_PORT=$DB_HOST_PORT"

# Indexer Settings (as used in templates)
INDEXER_API_PORT="${INDEXER_API_PORT:-3001}" # Host port for Indexer API

# Producer Settings (as used in templates)
PRODUCER_API_PORT="${PRODUCER_API_PORT:-8001}" # Host port for Producer API

# Internal RPC port for validators (used by indexer/producer templates)
VALIDATORS_RPC_PORT_START_INTERNAL="${VALIDATORS_RPC_PORT_START_INTERNAL:-26657}"

# Validate settings if Producer is enabled
if [ "$PRODUCER" == "true" ]; then
  if [ "$INDEXER" != "true" ]; then
    echo "Error: PRODUCER=true requires INDEXER=true."
    exit 1
  fi
  if [ -z "$PRODUCER_SEED_PHRASE" ]; then
    echo "Error: PRODUCER_SEED_PHRASE must be set when PRODUCER=true."
    exit 1
  fi
fi

ENV_L1="${APP_HOME}/.env"
L1_COMPOSE="${APP_HOME}/compose_l1.yaml"


if [ -d "${APP_HOME}" ]; then
    echo "Folder ${APP_HOME} already exist, need to delete it before running the script."
    read -p "Stop validators and Delete ${APP_HOME} folder??[y/N] " -n 1 -r
    echo    # (optional) move to a new line
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Stop if containers are already up
        [ $(docker ps |wc -l) -gt 1 ]  && docker compose -f $L1_COMPOSE down
        rm -rf "${APP_HOME}"
    fi
fi
mkdir -p "${APP_HOME}"


UID_GID="$(id -u):$(id -g)"
# echo "NETWORK_PREFIX=$NETWORK_PREFIX" >> ${ENV_L1}
echo "CHAIN_ID=$CHAIN_ID" >> ${ENV_L1}
echo "ALLORA_RPC=http://${NETWORK_PREFIX}.${VALIDATORS_IP_START}:26657" >> ${ENV_L1}  # Take validator0

echo "Copy generate_genesis.sh into localnet data directory so it can go into the container"
cp generate_genesis.sh "${APP_HOME}/generate_genesis.sh"

echo "Set permissions on data folder"
docker run --rm \
    -u 0:0 \
    -v "${APP_HOME}":/data \
    --entrypoint=chown \
    $DOCKER_IMAGE -R $(id -u):$(id -g) /data

docker run --rm \
    -u 0:0 \
    -v "${APP_HOME}":/data \
    --entrypoint=chmod \
    $DOCKER_IMAGE -R 777 /data

echo "Generate genesis and accounts"
docker run --rm \
    -u $(id -u):$(id -g) \
    -v "${APP_HOME}":/data \
    -v "$(pwd):/scripts" \
    -e APP_HOME=/data \
    -e HOME=/data \
    -e VALIDATOR_NUMBER=$VALIDATOR_NUMBER \
    --entrypoint=/data/generate_genesis.sh \
    $DOCKER_IMAGE

echo "Ensure permissions on data folder after genesis generation"
docker run --rm \
    -u 0:0 \
    -v "${APP_HOME}":/data \
    --entrypoint=chmod \
    $DOCKER_IMAGE -R 777 /data

echo "Updating expedited_voting_period in genesis.json"
genesis_file="${APP_HOME}/genesis/config/genesis.json"
tmp_file=$(mktemp)
jq '.app_state.gov.params.expedited_voting_period = "20s" | .app_state.gov.params.voting_period = "20s"' "$genesis_file" > "$tmp_file" && mv "$tmp_file" "$genesis_file"
echo "Updating remove_stake_delay_window in genesis.json"
jq '.app_state.emissions.params.remove_stake_delay_window = "10"' "$genesis_file" > "$tmp_file" && mv "$tmp_file" "$genesis_file"

echo "Generate L1 peers, put them in persisent-peers and in genesis.json"
PEERS=""
for ((i=0; i<$VALIDATOR_NUMBER; i++)); do
    valName="${VALIDATOR_PREFIX}${i}"
    ipAddress="${NETWORK_PREFIX}.$((VALIDATORS_IP_START+i))"
    addr=$(docker run --rm -t \
        -v "${APP_HOME}":/data \
        -u $(id -u):$(id -g) \
        --entrypoint=allorad \
        -e HOME=/data/data/${valName} \
        $DOCKER_IMAGE \
        --home=/data/data/${valName} tendermint show-node-id)
    addr="${addr%%[[:cntrl:]]}"
    delim=$([ $i -lt $(($VALIDATOR_NUMBER - 1)) ] && printf "," || printf "")
    PEERS="${PEERS}${addr}@${ipAddress}:26656${delim}"
done

echo "PEERS=$PEERS" >> ${ENV_L1}
echo "Generate docker compose file"
NETWORK_PREFIX=$NETWORK_PREFIX envsubst < compose_header.yaml > $L1_COMPOSE

for ((i=0; i<$VALIDATOR_NUMBER; i++)); do
    ipAddress="${NETWORK_PREFIX}.$((VALIDATORS_IP_START+i))" \
    moniker="${VALIDATOR_PREFIX}${i}" \
    validatorPort=$((VALIDATORS_RPC_PORT_START+i)) \
    validatorApiPort=$((VALIDATORS_API_PORT_START+i)) \
    validatorGrpcPort=$((VALIDATORS_GRPC_PORT_START+i)) \
    PEERS=$PEERS \
    NETWORK_PREFIX=$NETWORK_PREFIX \
    APP_HOME=$APP_HOME \
    UID_GID=$UID_GID \
    DOCKER_IMAGE=$DOCKER_IMAGE \
    envsubst < compose_templates/validator.yaml.tmpl >> $L1_COMPOSE
done

if [ "$INDEXER" == "true" ]; then
  echo "Adding PostgreSQL and Indexer services to Docker Compose from templates..."
  # Export variables needed by the templates for envsubst
  export POSTGRES_IMAGE_REF POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB_NAME DB_HOST_PORT
  export INDEXER_IMAGE_REF VALIDATOR_PREFIX VALIDATORS_RPC_PORT_START_INTERNAL CHAIN_ID INDEXER_API_PORT

  printf "\\n" >> $L1_COMPOSE 
  envsubst < compose_templates/database.yaml.tmpl >> $L1_COMPOSE
  
  printf "\\n" >> $L1_COMPOSE # Ensure separation
  envsubst < compose_templates/indexer.yaml.tmpl >> $L1_COMPOSE

  if [ "$PRODUCER" == "true" ]; then
    echo "Adding Producer service to Docker Compose from template..."
    export PRODUCER_IMAGE_REF PRODUCER_SEED_PHRASE PRODUCER_API_PORT

    printf "\\n" >> $L1_COMPOSE # Ensure separation
    envsubst < compose_templates/producer.yaml.tmpl >> $L1_COMPOSE
  fi
fi

echo "symlinking genesis.json to genesis/config/genesis.json"
rm -f "${APP_HOME}/genesis.json"

ln -s "${APP_HOME}/genesis/config/genesis.json" "${APP_HOME}/genesis.json"

echo "Launching the network"
cat $L1_COMPOSE
docker compose -f $L1_COMPOSE up -d --remove-orphans # Added --remove-orphans

echo "Waiting validator is up"
curl -o /dev/null --connect-timeout 5 \
    --retry 10 \
    --retry-delay 10 \
    --retry-all-errors \
    http://localhost:$VALIDATORS_RPC_PORT_START/status

echo "Checking the network is up and running"
heights=()
validators=()
for ((v=0; v<$VALIDATOR_NUMBER; v++)); do
    height=$(curl -s http://localhost:$((VALIDATORS_RPC_PORT_START+v))/status|jq -r .result.sync_info.latest_block_height)
    heights+=($height)
    echo "Got height: ${heights[$v]} from validator: http://localhost:$((VALIDATORS_RPC_PORT_START+v))"
    validators+=("localhost:$((VALIDATORS_RPC_PORT_START+v))")
    sleep 5
done

echo "Populate validators.json with validators addresses"
jq --compact-output --null-input '$ARGS.positional' --args -- "${validators[@]}" > "${APP_HOME}/validators.json"

chain_status=0
if [ ${#heights[@]} -eq $VALIDATOR_NUMBER ]; then
    for ((v=0; v<$((VALIDATOR_NUMBER-1)); v++)); do
        if [ ${heights[$v]} -lt ${heights[$((v+1))]} ]; then
            chain_status=$((chain_status+1))
        fi
    done
fi

docker run --rm -t \
    -u $(id -u):$(id -g) \
    -v "${APP_HOME}":/data \
    -e HOME=/data/genesis \
    --entrypoint=allorad \
    $DOCKER_IMAGE \
    --home /data/genesis config set client keyring-backend test

if [ $chain_status -eq $((VALIDATOR_NUMBER-1)) ]; then
    echo "Chain is up and running"
    echo
    echo "-----------------------------------------------------"
    echo "Local testnet with ${VALIDATOR_NUMBER} validator(s) is up and running."
    echo "-----------------------------------------------------"
    echo
    echo "Useful commands:"
    echo "  To view logs for all services: docker compose -f $L1_COMPOSE logs -f"
    echo "  To view logs for a specific validator (e.g., validator0): docker compose -f $L1_COMPOSE logs -f validator0"
    echo "  To stop all services: docker compose -f $L1_COMPOSE down"
    echo
    if [ "$INDEXER" == "true" ]; then
      echo "PostgreSQL (allora-db) is running and mapped to host port: ${DB_HOST_PORT}"
      echo "Indexer (allora-indexer) is running. API might be available at: http://localhost:${INDEXER_API_PORT}"
      echo "  To view Indexer logs: docker compose -f $L1_COMPOSE logs -f allora-indexer"
      echo "  To view Indexer Init logs: docker compose -f $L1_COMPOSE logs -f allora-indexer-init"
      if [ "$PRODUCER" == "true" ]; then
        echo "Producer (allora-producer) is running. API might be available at: http://localhost:${PRODUCER_API_PORT}"
        echo "  To view Producer logs: docker compose -f $L1_COMPOSE logs -f allora-producer"
      fi
      echo
    fi
    echo "Endpoints for the first validator (validator0):"
    echo "  RPC: http://localhost:${VALIDATORS_RPC_PORT_START}"
    echo "  API: http://localhost:${VALIDATORS_API_PORT_START}"
    echo "  gRPC: localhost:${VALIDATORS_GRPC_PORT_START}"
    echo "  (For other validators, increment the port numbers accordingly if they are exposed similarly)"
    echo
    echo "Example status check for validator0: curl http://localhost:${VALIDATORS_RPC_PORT_START}/status | jq ."
    echo "Example gRPC list for validator0: grpcurl -plaintext localhost:${VALIDATORS_GRPC_PORT_START} list"
    echo
    echo "To use allorad CLI commands against a validator (e.g., validator0):"
    echo "  allorad status --node tcp://localhost:${VALIDATORS_RPC_PORT_START} --home ${APP_HOME}/data/validator0"
    echo "  (Ensure your CLI is configured or specify --home for the respective validator's data directory, e.g., ${APP_HOME}/data/validator0)"
    echo
    echo "The main genesis file used by the network is at: ${APP_HOME}/genesis/config/genesis.json"
    echo "Individual validator data directories are in: ${APP_HOME}/data/validator[0-${VALIDATOR_NUMBER-1}]/"
    echo "-----------------------------------------------------"

else
    echo "Chain is not producing blocks"
    echo "If run locally you can check the logs with: docker logs allorad_validator_0"
    echo "and connect to the validators ..."
    exit 1
fi

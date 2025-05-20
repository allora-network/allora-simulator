#!/bin/bash
set -eu # Exit on error and undefined variables

# Ensure the script is executed from its directory
cd "$(dirname "$0")"

# --- Helper Functions ---

# Function to check if a command exists
check_command() {
  if ! command -v "$1" &> /dev/null; then
    echo "Error: $1 is not installed."
    exit 1
  fi
}

# --- Prerequisite Checks ---

# List of required commands
commands=("docker" "jq" "envsubst" "curl")

# Check each command
for cmd in "${commands[@]}"; do
  check_command "$cmd"
done

# --- Configuration Variables ---

# Docker image for Allora chain
DOCKER_IMAGE="alloranetwork/allora-chain:v0.11.0"

# Validator settings
VALIDATOR_NUMBER="${VALIDATOR_NUMBER:-3}" # Number of validators to run
VALIDATOR_PREFIX=validator                 # Prefix for validator container names
NETWORK_PREFIX="192.168.250"             # Docker network prefix
VALIDATORS_IP_START=10                     # Starting IP octet for validators
VALIDATORS_RPC_PORT_START=26657            # Starting RPC port for validators
VALIDATORS_API_PORT_START=1317             # Starting API port for validators
VALIDATORS_GRPC_PORT_START=9090            # Starting gRPC port for validators

# Head node settings (currently unused but defined)
HEADS_IP_START=20

# Chain settings
CHAIN_ID="${CHAIN_ID:-localnet}" # Chain ID for the local testnet
APP_HOME="$(pwd)/$CHAIN_ID"     # Application home directory for chain data

# Account settings
ACCOUNTS_TOKENS=1000000 # Tokens to allocate to generated accounts

# Indexer and Producer Settings
INDEXER="${INDEXER:-false}"                                 # Enable Indexer service
PRODUCER="${PRODUCER:-false}"                               # Enable Producer service
PRODUCER_SEED_PHRASE="${PRODUCER_SEED_PHRASE:-}"            # Seed phrase for Producer (mandatory if PRODUCER=true)

# Docker Image References for ancillary services (used in Docker Compose templates)
POSTGRES_IMAGE_REF="${POSTGRES_IMAGE_REF:-postgres:16-bookworm}"
INDEXER_IMAGE_REF="${INDEXER_IMAGE_REF:-alloranetwork/allora-indexer:v0.11.1}"
PRODUCER_IMAGE_REF="${PRODUCER_IMAGE_REF:-696230526504.dkr.ecr.us-east-1.amazonaws.com/allora-producer:v0.2.1}"

# Database Settings (used in Docker Compose templates)
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-password}"
POSTGRES_DB_NAME="${POSTGRES_DB_NAME:-alloradb_l1}"
DB_HOST_PORT="${DB_HOST_PORT:-5432}" # Host port mapping for the database

# Indexer Settings (used in Docker Compose templates)
INDEXER_API_PORT="${INDEXER_API_PORT:-3001}" # Host port mapping for Indexer API

# Producer Settings (used in Docker Compose templates)
PRODUCER_API_PORT="${PRODUCER_API_PORT:-8001}" # Host port mapping for Producer API

# Internal RPC port for validators (used by indexer/producer templates within Docker network)
VALIDATORS_RPC_PORT_START_INTERNAL="${VALIDATORS_RPC_PORT_START_INTERNAL:-26657}"

# --- Validate Settings ---

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

# --- Environment Setup ---

# Define paths for environment file and Docker Compose file
ENV_L1="${APP_HOME}/.env"
L1_COMPOSE="${APP_HOME}/compose_l1.yaml"

# Check if the application home directory exists and prompt for deletion if it does
if [ -d "${APP_HOME}" ]; then
    echo "Folder ${APP_HOME} already exists. It needs to be deleted before running the script."
    read -p "Stop validators and delete ${APP_HOME} folder? [y/N] " -n 1 -r
    echo # Move to a new line
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Stop Docker Compose services if they are running
        if [ "$(docker ps -q -f name=${VALIDATOR_PREFIX})" ]; then # Check if any validator container is running
            echo "Stopping existing Docker services..."
            docker compose -f "$L1_COMPOSE" down --remove-orphans
        fi
        echo "Removing ${APP_HOME}..."
        rm -rf "${APP_HOME}"
    else
        echo "Aborting script as ${APP_HOME} was not deleted."
        exit 1 # Exit if user chooses not to delete
    fi
fi
mkdir -p "${APP_HOME}" # Create the application home directory

# --- Prepare Environment File ---
echo "DB_HOST_PORT=$DB_HOST_PORT" # Add DB_HOST_PORT to .env for potential use by other scripts/services

# Get current user and group ID for Docker volume permissions
UID_GID="$(id -u):$(id -g)"

# Populate the .env file for Docker Compose
echo "CHAIN_ID=$CHAIN_ID" >> "${ENV_L1}"
# Set ALLORA_RPC to the first validator's RPC endpoint
echo "ALLORA_RPC=http://${NETWORK_PREFIX}.${VALIDATORS_IP_START}:${VALIDATORS_RPC_PORT_START}" >> "${ENV_L1}"

# --- Genesis and Account Generation ---

echo "Copying generate_genesis.sh into ${APP_HOME} for use within Docker container"
cp generate_genesis.sh "${APP_HOME}/generate_genesis.sh"

echo "Setting initial permissions on ${APP_HOME} folder for Docker user"
# Ensure the Docker user (root initially) can write to the mounted volume
docker run --rm \
    -u 0:0 \
    -v "${APP_HOME}":/data \
    --entrypoint=chown \
    "$DOCKER_IMAGE" -R "$UID_GID" /data

docker run --rm \
    -u 0:0 \
    -v "${APP_HOME}":/data \
    --entrypoint=chmod \
    "$DOCKER_IMAGE" -R 777 /data # Grant wide permissions temporarily

echo "Generating genesis file and accounts using Docker"
# This script (generate_genesis.sh) is expected to create necessary accounts and genesis file
docker run --rm \
    -u "$UID_GID" \
    -v "${APP_HOME}":/data \
    -v "$(pwd):/scripts" \
    -e APP_HOME=/data \
    -e HOME=/data \
    -e VALIDATOR_NUMBER="$VALIDATOR_NUMBER" \
    --entrypoint=/data/generate_genesis.sh \
    "$DOCKER_IMAGE"

echo "Ensuring permissions on ${APP_HOME} folder after genesis generation"
# Reset permissions to be safe, especially for files created by the container
docker run --rm \
    -u 0:0 \
    -v "${APP_HOME}":/data \
    --entrypoint=chmod \
    "$DOCKER_IMAGE" -R 777 /data # Grant wide permissions again

# --- Modify Genesis File ---

genesis_file="${APP_HOME}/genesis/config/genesis.json"
tmp_file=$(mktemp) # Create a temporary file for atomic updates

echo "Updating expedited_voting_period and voting_period in genesis.json"
jq '.app_state.gov.params.expedited_voting_period = "20s" | .app_state.gov.params.voting_period = "20s"' "$genesis_file" > "$tmp_file" && mv "$tmp_file" "$genesis_file"

echo "Updating remove_stake_delay_window in genesis.json"
tmp_file=$(mktemp) # Recreate tmp file for the next modification
jq '.app_state.emissions.params.remove_stake_delay_window = "10"' "$genesis_file" > "$tmp_file" && mv "$tmp_file" "$genesis_file"

# --- Generate Validator Peers ---

echo "Generating L1 peers and adding them to persisent_peers in config.toml and genesis.json"
PEERS=""
for ((i=0; i<VALIDATOR_NUMBER; i++)); do
    valName="${VALIDATOR_PREFIX}${i}"
    ipAddress="${NETWORK_PREFIX}.$((VALIDATORS_IP_START+i))"
    
    # Get the node ID for each validator
    addr=$(docker run --rm -t \
        -v "${APP_HOME}":/data \
        -u "$UID_GID" \
        --entrypoint=allorad \
        -e HOME="/data/data/${valName}" \
        "$DOCKER_IMAGE" \
        --home="/data/data/${valName}" tendermint show-node-id)
    addr="${addr%%[[:cntrl:]]}" # Remove any trailing control characters

    # Append to PEERS string, comma-separated
    delim=$([ "$i" -lt $((VALIDATOR_NUMBER - 1)) ] && printf "," || printf "")
    PEERS="${PEERS}${addr}@${ipAddress}:26656${delim}"
done

echo "PEERS=$PEERS" >> "${ENV_L1}" # Add generated peers to the .env file

# --- Generate Docker Compose File ---

echo "Generating Docker Compose file: ${L1_COMPOSE}"
# Start with the compose header
NETWORK_PREFIX="$NETWORK_PREFIX" envsubst < compose_header.yaml > "$L1_COMPOSE"

# Add validator services to the Docker Compose file
for ((i=0; i<VALIDATOR_NUMBER; i++)); do
    # Environment variables for the validator template
    ipAddress="${NETWORK_PREFIX}.$((VALIDATORS_IP_START+i))" \
    moniker="${VALIDATOR_PREFIX}${i}" \
    validatorPort=$((VALIDATORS_RPC_PORT_START+i)) \
    validatorApiPort=$((VALIDATORS_API_PORT_START+i)) \
    validatorGrpcPort=$((VALIDATORS_GRPC_PORT_START+i)) \
    PEERS="$PEERS" \
    NETWORK_PREFIX="$NETWORK_PREFIX" \
    APP_HOME="$APP_HOME" \
    UID_GID="$UID_GID" \
    DOCKER_IMAGE="$DOCKER_IMAGE" \
    envsubst < compose_templates/validator.yaml.tmpl >> "$L1_COMPOSE"
done

# Add Indexer and Producer services if enabled
if [ "$INDEXER" == "true" ]; then
  echo "Adding PostgreSQL and Indexer services to Docker Compose..."
  # Export variables needed by the templates for envsubst
  export POSTGRES_IMAGE_REF POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB_NAME DB_HOST_PORT
  export INDEXER_IMAGE_REF VALIDATOR_PREFIX VALIDATORS_RPC_PORT_START_INTERNAL CHAIN_ID INDEXER_API_PORT

  printf "\\n" >> "$L1_COMPOSE" # Ensure separation
  envsubst < compose_templates/database.yaml.tmpl >> "$L1_COMPOSE"
  
  printf "\\n" >> "$L1_COMPOSE" # Ensure separation
  envsubst < compose_templates/indexer.yaml.tmpl >> "$L1_COMPOSE"

  if [ "$PRODUCER" == "true" ]; then
    echo "Adding Producer service to Docker Compose..."
    export PRODUCER_IMAGE_REF PRODUCER_SEED_PHRASE PRODUCER_API_PORT

    printf "\\n" >> "$L1_COMPOSE" # Ensure separation
    envsubst < compose_templates/producer.yaml.tmpl >> "$L1_COMPOSE"
  fi
fi

# --- Final Preparations ---

echo "Symlinking ${APP_HOME}/genesis.json to ${APP_HOME}/genesis/config/genesis.json"
# Some tools might expect genesis.json at the root of APP_HOME
rm -f "${APP_HOME}/genesis.json" # Remove if it exists (e.g., from old runs)
ln -s "${APP_HOME}/genesis/config/genesis.json" "${APP_HOME}/genesis.json"

# --- Launch Network ---

echo "Launching the Allora testnet..."
echo "Docker Compose file content:"
cat "$L1_COMPOSE"
docker compose -f "$L1_COMPOSE" up -d --remove-orphans # Start services in detached mode

# --- Network Health Check ---

echo "Waiting for validator0 to be up..."
# Attempt to connect to the first validator's RPC endpoint
curl -o /dev/null --connect-timeout 5 \
    --retry 10 \
    --retry-delay 10 \
    --retry-all-errors \
    "http://localhost:${VALIDATORS_RPC_PORT_START}/status"

echo "Checking if the network is producing blocks..."
heights=()
validators=()
for ((v=0; v<VALIDATOR_NUMBER; v++)); do
    current_port=$((VALIDATORS_RPC_PORT_START+v))
    # Fetch the latest block height from each validator
    height=$(curl -s "http://localhost:${current_port}/status" | jq -r .result.sync_info.latest_block_height)
    heights+=("$height")
    echo "Validator $v (http://localhost:${current_port}): Block height = ${heights[$v]}"
    validators+=("localhost:${current_port}")
    sleep 5 # Wait a bit before querying the next validator
done

echo "Populating ${APP_HOME}/validators.json with validator addresses"
# Create a JSON file listing validator RPC endpoints for client use
jq --compact-output --null-input '$ARGS.positional' --args -- "${validators[@]}" > "${APP_HOME}/validators.json"

# Check if block heights are generally increasing (a simple health check)
chain_status=0
if [ ${#heights[@]} -eq "$VALIDATOR_NUMBER" ]; then
    all_heights_valid=true
    for height_val in "${heights[@]}"; do
        if ! [[ "$height_val" =~ ^[0-9]+$ ]] || [ "$height_val" -le 0 ]; then
            all_heights_valid=false
            echo "Warning: Invalid or non-positive block height '$height_val' detected for a validator."
            break
        fi
    done

    if [ "$all_heights_valid" = true ] && [ "$VALIDATOR_NUMBER" -gt 1 ]; then
        for ((v=0; v<$((VALIDATOR_NUMBER-1)); v++)); do
            # Check if current height is less than or equal to next to infer progression
            # This logic assumes heights should be generally increasing or at least not decreasing significantly
            # A more robust check would be to see if new blocks are produced over a time window.
            if [ "${heights[$v]}" -le "${heights[$((v+1))]}" ]; then # Simplified check, could be more robust
                chain_status=$((chain_status+1))
            fi
        done
    elif [ "$all_heights_valid" = true ] && [ "$VALIDATOR_NUMBER" -eq 1 ]; then
        # If only one validator, and height is valid, consider it 'progressing' for this simple check
         if [ "${heights[0]}" -gt 0 ]; then
            chain_status=1 # Mark as healthy for single validator case
         fi
    fi
fi


echo "Setting keyring-backend to test for allorad CLI"
# Configure the CLI to use the 'test' keyring backend for easier scripting (no password prompts)
docker run --rm -t \
    -u "$UID_GID" \
    -v "${APP_HOME}":/data \
    -e HOME=/data/genesis \
    --entrypoint=allorad \
    "$DOCKER_IMAGE" \
    --home /data/genesis config set client keyring-backend test

# --- Output Network Information ---

# Check if the chain is considered up based on the chain_status heuristic
# For multiple validators, chain_status should be VALIDATOR_NUMBER - 1
# For a single validator, chain_status should be 1 (as set above)
expected_chain_status=$((VALIDATOR_NUMBER > 1 ? VALIDATOR_NUMBER - 1 : (VALIDATOR_NUMBER == 1 ? 1 : 0) ))

if [ "$chain_status" -ge "$expected_chain_status" ] && [ "$VALIDATOR_NUMBER" -gt 0 ]; then
    echo
    echo "----------------------------------------------------------------------"
    echo "Local Allora testnet (${VALIDATOR_NUMBER} validator(s)) is UP and RUNNING!"
    echo "----------------------------------------------------------------------"
    echo
    echo "Validator0 Endpoints:"
    echo "  RPC:  http://localhost:${VALIDATORS_RPC_PORT_START}"
    echo "  API:  http://localhost:${VALIDATORS_API_PORT_START}"
    echo "  gRPC: localhost:${VALIDATORS_GRPC_PORT_START}"
    echo

    if [ "$INDEXER" == "true" ]; then
      echo "Indexer Service (Enabled):"
      echo "  API: http://localhost:${INDEXER_API_PORT}"
      if [ "$PRODUCER" == "true" ]; then
        echo "Producer Service (Enabled):"
        echo "  API: http://localhost:${PRODUCER_API_PORT}"
      fi
      echo
    fi
else
    echo
    echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
    echo "ERROR: Allora testnet FAILED to start or is not healthy."
    echo "  Validators: ${VALIDATOR_NUMBER} | Progressing status: ${chain_status} (expected >= ${expected_chain_status})"
    echo
    echo "Troubleshooting:"
    echo "  View all logs:         docker compose -f \"${L1_COMPOSE}\" logs -f"
    echo "  View validator0 logs:  docker logs \"${VALIDATOR_PREFIX}0\" (or similar name from 'docker ps')"
    echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
    exit 1
fi

exit 0

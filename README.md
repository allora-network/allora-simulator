# Allora Simulator

## Overview

The Allora Simulator provides three main modules:
- **Stress Module**: Tests network performance and correctness under various conditions
- **Research Module**: Simulates specific scenarios with controlled parameters for research purposes
- **Basic Activity**: Simulates basic network activity such as sending tokens

## Prerequisites

- Go 1.19 or later
- Access to an Allora Network node
- Basic understanding of the Allora Network protocol

## Installation

```bash
git clone https://github.com/allora-network/allora-simulator
cd allora-simulator
make setup
```
The setup command will:
- Copy `config.example.json` to `config.json`
- Copy `.env.example` to `.env`
- Run `go mod tidy`

## Configuration

### Basic Setup
- Copy `config.example.json` to `config.json`
- Copy `.env.example` to `.env`
- Adjust parameters as needed

### Configuration Parameters

#### Common Parameters
```json
{
    "chain_id": "localnet",
    "denom": "uallo",
    "prefix": "allo",
    "gas_per_byte": 100,
    "base_gas": 2000000,
    "epoch_length": 12,
    "num_topics": 1,
    "inferers_per_topic": 5,
    "forecasters_per_topic": 5,
    "reputers_per_topic": 3,
    "create_topics_same_block": false,
    "timeout_minutes": 30,
    "nodes": {
        "rpc": ["http://127.0.0.1:26657"],
        "api": "http://localhost:1317",
        "grpc": "localhost:9090"
    }
}
```

#### Research Module Parameters
```json
{
    "research": {
        "initial_price": 100.0,
        "drift": 0.0001,
        "volatility": 0.02,
        "base_experience_factor": 0.5,
        "experience_growth": 0.1,
        "outperform_value": 0.5,
        "consistent_outperformer": false,
        "topic": {
            "loss_method": "mse",
            "epoch_length": 12,
            "ground_truth_lag": 12,
            "worker_submission_window": 10,
            "p_norm": "3.0",
            "alpha_regret": "0.1",
            "allow_negative": false,
            "epsilon": "0.01",
            "merit_sortition_alpha": "0.1",
            "active_inferer_quantile": "0.25",
            "active_forecaster_quantile": "0.25",
            "active_reputer_quantile": "0.25"
        },
        "global_params": {
            "max_samples_to_scale_scores": 10
        }
    }
}
```

#### Basic Activity Module Parameters
```json
{
  "basic_activity": {
    "num_actors": 15,
    "rand_wallet_seed": 12345,
    "txs_per_block": {
      "min": 5,
      "max": 10
    },
    "send_amount": {
      "min": "1000000000000000000",
      "max": "15000000000000000000"
    }
  }
}
```

## Running the Simulator

### Step 1 - Chain Setup

Choose one of these options:

#### Option A: Setting up a New Local Testnet

To start a new local testnet, use the `make localnet` command. This command utilizes the `./scripts/local_testnet.sh` script to set up and run the network using Docker Compose.

```bash
make localnet
```

**Customizing the Testnet:**

You can control the testnet configuration using environment variables:

*   `VALIDATOR_NUMBER`: Specifies the number of validator nodes to create. For example, to start a testnet with 5 validators:
    ```bash
    VALIDATOR_NUMBER=5 make localnet
    ```
    If not set, it defaults to the script's internal default (e.g., 3 validators).

*   `INDEXER`: Set to `true` to include an indexer service in the testnet. If enabled, one instance of the indexer will be started and connected to the first validator (`validator0`).
    ```bash
    INDEXER=true make localnet
    ```

*   `PRODUCER`: Set to `true` to include a producer service in the testnet. If enabled, one instance of the producer will be started.
    ```bash
    PRODUCER=true make localnet
    ```

You can combine these variables:
```bash
VALIDATOR_NUMBER=2 INDEXER=true PRODUCER=true make localnet
```

**Default Configuration:**

The local testnet should be accessible via the default RPC/API/gRPC ports (e.g., `validator0` at `http://127.0.0.1:26657` for RPC). Ensure your `config.json` reflects this if you intend to connect the simulator to this localnet:
```json
{
    "nodes": {
        "rpc": ["http://127.0.0.1:26657"],
        "api": "http://localhost:1317",
        "grpc": "localhost:9090"
    }
}
```

To stop and clean up the local testnet (removing all associated Docker containers, networks, and volumes), run:
```bash
make localnet-stop
```

#### Option B: Using Existing Chain
1. Add your funded account's seedphrase:
   ```bash
   echo "your seed phrase here" > scripts/seedphrase
   ```

2. Update config.json with chain parameters:
   ```json
   {
       "chain_id": "your_chain_id",
       "denom": "your_denom",
       "prefix": "your_prefix",
       "nodes": {
           "rpc": ["your_rpc_endpoint"],
           "api": "your_api_endpoint",
           "grpc": "your_grpc_endpoint"
       }
   }
   ```

### Step 2 - Running Modules

After setting up the chain, you can run either module:

#### Stress Testing Module
```bash
make stress
```
Use this to:
- Test network performance under load
- Create multiple topics
- Run concurrent worker and reputer operations

#### Research Module
```bash
make research
```
Use this to:
- Run controlled experiments
- Simulate specific market conditions

#### Basic Activity Module
```bash
make basic
```
Use this to:
- Simulate sending tokens between accounts

### Step 3 - Chaos Testing with Pumba (Optional)

After starting your local testnet (`make localnet`), you can inject network disturbances into validator nodes using Pumba.

For more information about Pumba, see the official repository: https://github.com/alexei-led/pumba

You can customize the Pumba tests using the following make variables:

- `CHAOS_TARGETS`: Specifies one or more target validator containers, separated by spaces (e.g., "validator0 validator1").
- `DURATION_S`: Duration of the Pumba effect in seconds.
- `LOSS_PERCENT`: Percentage of packet loss (0-100) for chaos-loss.
- `DELAY_MS`: Network delay in milliseconds for chaos-delay.

#### Available Chaos Commands

**Inject Packet Loss (chaos-loss):**
Simulates a percentage of network packets being lost for the target validator(s).

```bash
# Target validator0 with 10% loss for 60s (defaults)
make chaos-loss

# Target validator1 with 30% loss for 120s
make chaos-loss CHAOS_TARGETS=validator1 LOSS_PERCENT=30 DURATION_S=120

# Target validator0 and validator2 with 50% loss for 60s
make chaos-loss CHAOS_TARGETS="validator0 validator2" LOSS_PERCENT=50
```

**Add Network Delay (chaos-delay):**
Simulates network latency for the target validator(s).

```bash
# Target validator0 with 200ms delay for 60s (defaults)
make chaos-delay

# Target validator1 with 500ms delay for 30s
make chaos-delay CHAOS_TARGETS=validator1 DELAY_MS=500 DURATION_S=30

# Target validator0 and validator2 with 1000ms delay for 60s
make chaos-delay CHAOS_TARGETS="validator0 validator2" DELAY_MS=1000
```

## Module Differences

### Stress Module
- Creates multiple topics
- Combines inferers and forecasters as workers
- Focuses on network performance and stability
- Uses simplified random value generation

### Research Module
- Creates a single topic
- Separates inferers and forecasters
- Uses sophisticated price simulation
- Tracks actor experience and performance
- Supports outperformance scenarios

### Basic Activity Module
- Creates deterministic random wallets
- Send random amount of txs per block according to the configured range
- Send random number of tokens per tx according to the configured range

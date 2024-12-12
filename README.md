# Allora Simulator

## Overview

The Allora Simulator provides two main modules:
- **Stress Module**: Tests network performance and correctness under various conditions
- **Research Module**: Simulates specific scenarios with controlled parameters for research purposes

## Prerequisites

- Go 1.19 or later
- Access to an Allora Network node
- Basic understanding of the Allora Network protocol

## Installation

```bash
git clone https://github.com/allora-network/allora-simulator
cd allora-simulator
go mod tidy
```

## Configuration

### Basic Setup
- Copy `config.example.json` to `config.json`
- Copy `.env.example` to `.env`
- Adjust parameters as needed

### Configuration Parameters

#### Common Parameters
```json
{
    "chain_id": "demo",
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
        }
    }
}
```

## Running the Simulator

### Step 1 - Chain Setup

Choose one of these options:

#### Option A: Using Docker Compose (New Local Chain)
1. Create a local chain:
   ```bash
   docker compose up
   ```
2. The script will:
   - Generate a faucet account
   - Save the seedphrase in `scripts/seedphrase`
   - Create a local chain with default parameters

3. Update config.json for local chain:
   ```json
   {
       "nodes": {
           "rpc": ["http://127.0.0.1:26657"],
           "api": "http://localhost:1317",
           "grpc": "localhost:9090"
       }
   }
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
go run cmd/stress/main.go
```
Use this to:
- Test network performance under load
- Create multiple topics
- Run concurrent worker and reputer operations

#### Research Module
```bash
go run cmd/research/main.go
```
Use this to:
- Run controlled experiments
- Simulate specific market conditions

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

# Allora Simulator

## Overview

The Allora Simulator is designed to:
- Test correctness in different scenarios
- Test network performance under various conditions

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

## Configuring 
- Copy `config.example.json` to `config.json`
- Copy `.env.example` to `.env`
- Adjust parameters as needed

## Running the Simulator

There are two ways to run the simulator:

### Method 1: Using Docker Compose (Creating a New Chain)

This method automatically sets up a local node and creates the necessary accounts:

1. Ensure you have created the `.env` file from the example
2. Run the following command to start the simulator:
   ```bash
   docker compose up
   ```
3. The script will automatically generate a faucet account and save its seedphrase in the `scripts/seedphrase` file.

### Method 2: Pointing to an Existing Chain

If you want to run the simulator against an existing chain:

1. Create a `seedphrase` file inside the `scripts` folder with your funded wallet's mnemonic
2. Ensure the wallet has sufficient funds for all simulated actors
3. Run the simulator:
   ```bash
   go run main.go
   ```
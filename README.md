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
- Adjust parameters as needed (especially node endpoints)

## Set Up Faucet Account
- Create `seedphrase` file with your funded wallet's mnemonic
- Ensure wallet has sufficient funds for all simulated actors

## Run the Simulator
```bash
go run main.go
```
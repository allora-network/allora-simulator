.PHONY: setup stress research localnet localnet-stop

setup:
	cp config.example.json config.json
	cp .env.example .env
	go mod tidy

stress:
	go run cmd/stress/main.go

research:
	go run cmd/research/main.go

# Starts a local L1 testnet using a script
localnet:
	VALIDATOR_NUMBER=3 INDEXER=true ./scripts/local_testnet_l1.sh

# Stops and cleans the local L1 testnet
localnet-stop:
	docker compose -f ./scripts/localnet/compose_l1.yaml down -v

.PHONY: setup stress research localnet localnet-stop

# Setup the project
setup:
	cp config.example.json config.json
	cp .env.example .env
	go mod tidy

# Run the stress mode
stress:
	go run cmd/stress/main.go

# Run the research mode
research:
	go run cmd/research/main.go

# Run the basic activity mode
basic:
	go run cmd/basic_activity/main.go

# Starts a local L1 testnet using a script
localnet:
	@export VALIDATOR_NUMBER=$${VALIDATOR_NUMBER:-3}; \
	export INDEXER=$${INDEXER:-false}; \
	export PRODUCER=$${PRODUCER:-false}; \
	./scripts/local_testnet.sh

# Stops and cleans the local L1 testnet
localnet-stop:
	docker compose -f ./scripts/localnet/compose_l1.yaml down -v

# --- CHAOS TESTING TARGETS ---
# Target validator(s), space-separated (e.g., "validator0 validator1")
CHAOS_TARGETS  ?=validator0 
# Duration of the chaos effect in seconds
DURATION_S     ?=60
# Packet loss percentage (0-100)
LOSS_PERCENT   ?=10
# Network delay in milliseconds
DELAY_MS       ?=200

# Inject packet loss into CHAOS_TARGETS
chaos-loss:
	@echo "Injecting $(LOSS_PERCENT)% packet loss into [$(CHAOS_TARGETS)] for $(DURATION_S) seconds..."
	docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock gaiaadm/pumba netem --duration $(DURATION_S)s --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest loss --percent $(LOSS_PERCENT) $(CHAOS_TARGETS)
	@echo "Packet loss injection finished for [$(CHAOS_TARGETS)]."

# Add network delay to CHAOS_TARGETS
chaos-delay:
	@echo "Adding $(DELAY_MS)ms network delay to [$(CHAOS_TARGETS)] for $(DURATION_S) seconds..."
	docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock gaiaadm/pumba netem --duration $(DURATION_S)s --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest delay --time $(DELAY_MS) $(CHAOS_TARGETS)
	@echo "Network delay injection finished for [$(CHAOS_TARGETS)]."

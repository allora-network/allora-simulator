.PHONY: setup stress research

setup:
	cp config.example.json config.json
	cp .env.example .env
	go mod tidy

docker:
	@if [ -d "data" ]; then \
		read -p "Data directory exists. Do you want to delete it? [y/N] " answer; \
		if [ "$$answer" = "y" ] || [ "$$answer" = "Y" ]; then \
			rm -rf data/*; \
			echo "Data directory cleaned"; \
		else \
			echo "Keeping existing data"; \
		fi \
	fi
	docker compose up -d

stress:
	go run cmd/stress/main.go

research:
	go run cmd/research/main.go

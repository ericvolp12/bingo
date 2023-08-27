GO_CMD = CGO_ENABLED=0 GOOS=linux go

.PHONY: server
server:
	$(GO_CMD) build -o bin/server ./cmd/server

.PHONY: up
up:
	docker-compose up --build  -d

# Generate SQLC Code
.PHONY: sqlc
sqlc:
	@echo "Generating SQLC code..."
	sqlc generate -f pkg/store/sqlc.yaml

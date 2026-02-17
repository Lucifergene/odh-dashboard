# Makefile for gen-ai development
DEFAULT_ENV_FILE := .env
ifneq ("$(wildcard $(DEFAULT_ENV_FILE))","")
include ${DEFAULT_ENV_FILE}
export $(shell sed 's/=.*//' ${DEFAULT_ENV_FILE})
endif

DEV_ENV_FILE := .env.development
ifneq ("$(wildcard $(DEV_ENV_FILE))","")
include ${DEV_ENV_FILE}
export $(shell sed 's/=.*//' ${DEV_ENV_FILE})
endif

ENV_FILE := .env.local
ifneq ("$(wildcard $(ENV_FILE))","")
include ${ENV_FILE}
export $(shell sed 's/=.*//' ${ENV_FILE})
endif

PORT_FORWARD_PID_FILE := /tmp/gen-ai-portforward.pids

.PHONY: help dev-install-dependencies dev-frontend dev-bff dev-start dev-start-no-portforward dev-portforward dev-stop-portforward clean clean-all

help: ## Display this help message
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*##"; printf "\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: dev-install-dependencies
dev-install-dependencies: ## Install dependencies for frontend
	cd frontend && npm install

.PHONY: dev-bff-mock
dev-bff-mock: ## Run BFF with all mocks enabled (K8s, LlamaStack, MaaS, MCP, MLflow)
	cd bff && make run LOG_LEVEL=debug MOCK_LS_CLIENT=true MOCK_K8S_CLIENT=true MOCK_MAAS_CLIENT=true MOCK_MCP_CLIENT=true MOCK_MLFLOW_CLIENT=true AUTH_TOKEN=FAKE_BEARER_TOKEN

.PHONY: dev-bff
dev-bff:
	cd bff && MAAS_URL=${MAAS_URL} DISTRIBUTION_NAME=${DISTRIBUTION_NAME} LLAMA_STACK_URL=${LLAMA_STACK_URL} AUTH_TOKEN=user_token AUTH_TOKEN_HEADER=Authorization AUTH_TOKEN_PREFIX="Bearer " make run LOG_LEVEL=debug STATIC_ASSETS_DIR=../frontend/dist

.PHONY: dev-bff-debug
dev-bff-debug: ## Run BFF with Delve debugger (VSCode can attach - requires Delve installation, see readme)
	cd bff && MAAS_URL=${MAAS_URL} DISTRIBUTION_NAME=${DISTRIBUTION_NAME} LLAMA_STACK_URL=${LLAMA_STACK_URL} AUTH_TOKEN=user_token AUTH_TOKEN_HEADER=Authorization AUTH_TOKEN_PREFIX="Bearer " make debug LOG_LEVEL=debug STATIC_ASSETS_DIR=../frontend/dist

.PHONY: dev-frontend
dev-frontend: ## Run only the frontend in development mode
	cd frontend && npm run start:dev

.PHONY: dev-mlflow-up
dev-mlflow-up: ## Start local MLflow tracking server for development
	cd bff && make mlflow-up

.PHONY: dev-start-mock
dev-start-mock: ## Run frontend and bff (all mocks) in development mode
	make -j 2 dev-bff-mock dev-frontend

.PHONY: dev-portforward
dev-portforward: dev-stop-portforward ## Start port-forwarding with auto-reconnect
	@echo "Starting port-forward for maas-api (auto-reconnect)..."
	@(while true; do oc port-forward -n ${MAAS_NAMESPACE} svc/maas-api 8081:8080 2>&1; echo "[maas-api] port-forward dropped, reconnecting in 3s..."; sleep 3; done) & echo $$! >> $(PORT_FORWARD_PID_FILE)
	@sleep 1
	@echo "Starting port-forward for lsd-genai-playground-service (auto-reconnect)..."
	@(while true; do oc port-forward -n ${PLAYGROUND_NAMESPACE} svc/lsd-genai-playground-service 8321:8321 2>&1; echo "[lsd-playground] port-forward dropped, reconnecting in 3s..."; sleep 3; done) & echo $$! >> $(PORT_FORWARD_PID_FILE)
	@echo "Port-forwarding started in background (PIDs stored in $(PORT_FORWARD_PID_FILE))"

.PHONY: dev-stop-portforward
dev-stop-portforward: ## Stop any running port-forward loops
	@if [ -f $(PORT_FORWARD_PID_FILE) ]; then \
		echo "Stopping previous port-forward processes..."; \
		while read -r pid; do \
			kill -TERM -- -$$pid 2>/dev/null || kill $$pid 2>/dev/null || true; \
		done < $(PORT_FORWARD_PID_FILE); \
		rm -f $(PORT_FORWARD_PID_FILE); \
		echo "Stopped."; \
	fi

.PHONY: dev-start
dev-start: dev-portforward ## Start frontend, bff, and port-forwarding
	@trap '$(MAKE) dev-stop-portforward' EXIT; \
	sleep 2; \
	$(MAKE) -j 2 dev-bff dev-frontend

.PHONY: dev-start-debug
dev-start-debug: dev-portforward ## Start frontend, bff with debugger, and port-forwarding (VSCode can attach on port 2345)
	@trap '$(MAKE) dev-stop-portforward' EXIT; \
	sleep 2; \
	echo ""; \
	echo "🐛 Starting in DEBUG mode..."; \
	echo "📍 Delve debugger will listen on port 2345"; \
	echo "📍 In VSCode: Press F5 and select 'Attach to Delve (make dev-start-debug)'"; \
	echo ""; \
	$(MAKE) -j 2 dev-bff-debug dev-frontend

.PHONY: dev-start-no-portforward
dev-start-no-portforward: ## Start frontend and bff without port-forwarding
	make -j 2 dev-bff dev-frontend

clean: dev-stop-portforward ## Clean build artifacts and node_modules
	@echo "Cleaning root dist..."
	npm run clean 2>/dev/null || true
	@echo "Cleaning frontend dist..."
	cd frontend && npm run clean 2>/dev/null || true
	@echo "Cleaning bff..."
	cd bff && make clean 2>/dev/null || true
	@echo "Clean complete!"

clean-all: clean ## Clean everything including node_modules
	@echo "Cleaning node_modules..."
	rm -rf node_modules 2>/dev/null || true
	cd frontend && rm -rf node_modules 2>/dev/null || true
	@echo "Full clean complete!"

.DEFAULT_GOAL := help

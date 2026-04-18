BINARY_NAME := flowmancer-agent
GO_PACKAGE := ./cmd/agent
BPF_GEN_PACKAGE := ./internal/ebpfgen

# bpf2go generated files (prefix: flow)
GENERATED_FILES := \
	flow_bpfel.go \
	flow_bpfeb.go \
	flow_bpfel.o \
	flow_bpfeb.o

.PHONY: all generate build run clean help

all: build

generate: ## Generate Go bindings from eBPF C code
	@echo ">> Generating eBPF code..."
	go generate $(BPF_GEN_PACKAGE)

build: generate ## Build the Flowmancer agent
	@echo ">> Building $(BINARY_NAME)..."
	go build -buildvcs=false -o $(BINARY_NAME) $(GO_PACKAGE)

run: build ## Run the agent (requires root)
	@echo ">> Running $(BINARY_NAME)..."
	sudo ./$(BINARY_NAME)

clean: ## Clean build artifacts
	@echo ">> Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f $(GENERATED_FILES)

help: ## Show help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
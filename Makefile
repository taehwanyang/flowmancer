BINARY_NAME := flowmancer-agent
GO_PACKAGE := ./cmd/agent
BPF_GEN_PACKAGE := ./internal/ebpfgen

# bpf2go generated files
GENERATED_FILES := \
	$(BPF_GEN_PACKAGE)/flow_bpfel.go \
	$(BPF_GEN_PACKAGE)/flow_bpfeb.go \
	$(BPF_GEN_PACKAGE)/flow_bpfel.o \
	$(BPF_GEN_PACKAGE)/flow_bpfeb.o \
	$(BPF_GEN_PACKAGE)/dns_bpfel.go \
	$(BPF_GEN_PACKAGE)/dns_bpfeb.go \
	$(BPF_GEN_PACKAGE)/dns_bpfel.o \
	$(BPF_GEN_PACKAGE)/dns_bpfeb.o

.PHONY: all generate build run clean help

all: build

generate: ## Generate Go bindings from eBPF C sources
	@echo ">> Generating eBPF code..."
	go generate $(BPF_GEN_PACKAGE)

build: generate ## Build the Flowmancer agent
	@echo ">> Building $(BINARY_NAME)..."
	go build -buildvcs=false -o $(BINARY_NAME) $(GO_PACKAGE)

run: build ## Run the agent (requires root, preserves env)
	@echo ">> Running $(BINARY_NAME)..."
	sudo -E ./$(BINARY_NAME)

clean: ## Clean build artifacts
	@echo ">> Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f $(GENERATED_FILES)

help: ## Show help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
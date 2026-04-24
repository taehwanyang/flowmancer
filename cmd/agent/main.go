package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/taehwanyang/flowmancer/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	configPath := resolveConfigPath()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	log.Printf("[config] using config file: %s", configPath)

	app, err := NewApp(cfg)
	if err != nil {
		log.Fatalf("new app: %v", err)
	}

	if err := app.Run(ctx); err != nil {
		log.Fatalf("run app: %v", err)
	}
}

func resolveConfigPath() string {
	if path := os.Getenv("FLOWMANCER_CONFIG"); path != "" {
		return path
	}

	if _, err := os.Stat("./flowmancer-agent-config.yaml"); err == nil {
		return "./flowmancer-agent-config.yaml"
	}

	return "/etc/flowmancer/flowmancer-agent-config.yaml"
}

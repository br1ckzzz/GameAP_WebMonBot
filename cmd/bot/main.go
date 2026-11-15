package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gameap-webmon-bot/internal/bot"
	"gameap-webmon-bot/internal/config"
	"gameap-webmon-bot/internal/gameap"
)

const AppName = "GameAP WebMonBot"

var AppVersion = "1.0.0-rc1"

func main() {
	configPath := flag.String("config", "", "Path to configuration JSON file (defaults to config.json)")
	statePath := flag.String("state", "state.json", "Path to state JSON file")
	checkOnly := flag.Bool("check", false, "Test connection to GameAP WebMonitoring API and exit")
	showVersion := flag.Bool("version", false, "Print bot version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s v%s\n", AppName, AppVersion)
		os.Exit(0)
	}

	fmt.Println("===========================================")
	fmt.Printf("  %s v%s\n", AppName, AppVersion)
	fmt.Println("  Discord Live Monitoring for GameAP")
	fmt.Println("===========================================")

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to load configuration: %v", err)
	}

	resolvedStatePath := *statePath
	if resolvedStatePath == "state.json" {
		if _, err := os.Stat("state.json"); err != nil {
			if _, parentErr := os.Stat("../state.json"); parentErr == nil {
				resolvedStatePath = "../state.json"
			}
		}
	}

	log.Printf("[CONFIG] GameAP Base URL: %s", cfg.GameAP.BaseURL)
	log.Printf("[CONFIG] Update Interval: %d seconds", cfg.Bot.UpdateIntervalSeconds)
	if cfg.GameAP.APIToken != "" {
		log.Printf("[CONFIG] Bot API Token: [configured, %d chars]", len(cfg.GameAP.APIToken))
	} else {
		log.Printf("[CONFIG] Bot API Token: [not set - using public routes]")
	}

	// If check flag is provided, test connection to GameAP WebMonitoring
	if *checkOnly {
		log.Println("[CHECK] Testing connection to GameAP WebMonitoring...")
		client := gameap.NewClient(cfg.GameAP.BaseURL, cfg.GameAP.APIToken, 10*time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := client.FetchServers(ctx)
		if err != nil {
			log.Fatalf("[CHECK] FAILED: %v", err)
		}

		online := 0
		for _, s := range resp.Servers {
			if s.Status == "online" {
				online++
			}
		}

		log.Printf("[CHECK] SUCCESS! Found %d total servers (%d online).", len(resp.Servers), online)
		os.Exit(0)
	}

	if cfg.Bot.Token == "" || cfg.Bot.Token == "YOUR_DISCORD_BOT_TOKEN_HERE" {
		log.Println("[WARN] No valid Discord bot token found!")
		log.Println("[WARN] Please set your bot token in config.json or export DISCORD_BOT_TOKEN.")
		log.Println("[INFO] You can run with -check to test GameAP WebMonitoring API connectivity without Discord.")
		os.Exit(1)
	}

	b, err := bot.NewBot(cfg, resolvedStatePath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize bot: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown on SIGINT / SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n[INFO] Shutdown signal received, closing sessions...")
		cancel()
		_ = b.Stop()
		os.Exit(0)
	}()

	if err := b.Start(ctx); err != nil {
		log.Fatalf("[FATAL] Bot error: %v", err)
	}
}

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"bandiq-node-agent/internal/identity"
	"bandiq-node-agent/internal/verifier"
)

func main() {
	port := flag.Int("port", 8085, "HTTP port for the verifier service")
	keyPath := flag.String("key", "verifier_key.json", "Path to verifier private key file")
	capGB := flag.Uint64("daily-cap-gb", 10, "Daily data volume cap per node in gigabytes")
	confidenceThreshold := flag.Float64("confidence", 0.70, "Minimum confidence threshold to accept reports (0.0 - 1.0)")
	maxDriftMin := flag.Int("max-drift-min", 5, "Maximum allowable clock drift in minutes")
	flag.Parse()

	// 1. Initialize Verifier Cryptographic Identity
	var verifierID *identity.Identity
	if _, err := os.Stat(*keyPath); err == nil {
		loadedID, err := identity.LoadFromFile(*keyPath)
		if err != nil {
			log.Fatalf("Failed to load verifier key from %s: %v", *keyPath, err)
		}
		verifierID = loadedID
		log.Printf("[Verifier] Loaded existing verifier identity: %s", verifierID.NodeID)
	} else {
		genID, err := identity.GenerateNewIdentity("0x0000000000000000000000000000000000000001")
		if err != nil {
			log.Fatalf("Failed to generate verifier keypair: %v", err)
		}
		if err := genID.SaveToFile(*keyPath); err != nil {
			log.Fatalf("Failed to persist verifier key: %v", err)
		}
		verifierID = genID
		absPath, _ := filepath.Abs(*keyPath)
		log.Printf("[Verifier] Generated new verifier identity: %s (saved to %s)", verifierID.NodeID, absPath)
	}

	// 2. Setup Verification Pipeline Configuration
	cfg := verifier.DefaultPipelineConfig()
	cfg.DailyDataCapBytes = *capGB * 1024 * 1024 * 1024
	cfg.ConfidenceThreshold = *confidenceThreshold
	cfg.MaxClockDrift = time.Duration(*maxDriftMin) * time.Minute

	service := verifier.NewService(verifierID, cfg)

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("==================================================================")
	log.Printf("       BANDIQ Proof-of-Bandwidth Verification Service v0.1.0      ")
	log.Printf("==================================================================")
	log.Printf("  Verifier Identity:      %s", verifierID.NodeID)
	log.Printf("  Verifier Public Key:    %s", verifierID.PublicKeyHex())
	log.Printf("  Listening Address:      http://%s", addr)
	log.Printf("  Verification Gates:     10 Sequential Integrity Gates")
	log.Printf("  Confidence Threshold:   %.2f", cfg.ConfidenceThreshold)
	log.Printf("  Daily Quota Limit:      %d GB / node", *capGB)
	log.Printf("  Clock Drift Window:     ±%v", cfg.MaxClockDrift)
	log.Printf("==================================================================")

	// Graceful shutdown handling
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := service.StartHTTP(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Verifier HTTP server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("[Verifier] Shutdown signal received, terminating gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = service.Stop(shutdownCtx)

	log.Println("[Verifier] Service safely stopped.")
}

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"bandiq-node-agent/internal/admin"
	"bandiq-node-agent/internal/blockchain"
	"bandiq-node-agent/internal/client"
	"bandiq-node-agent/internal/config"
	"bandiq-node-agent/internal/consent"
	"bandiq-node-agent/internal/identity"
	"bandiq-node-agent/internal/measure"
	"bandiq-node-agent/internal/report"
	"bandiq-node-agent/internal/spool"
	"bandiq-node-agent/internal/system"
)

const (
	banner = `
===========================================================
  BANDIQ DePIN Bandwidth Node Agent (v%s)
  Network: Sidra Chain (ID: %d)
  Token:   %s ($BNDQ)
===========================================================
`
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		runInit(os.Args[2:])
	case "consent":
		runConsent(os.Args[2:])
	case "start":
		runStart(os.Args[2:])
	case "test":
		runTest(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "pause":
		runPause(os.Args[2:])
	case "resume":
		runResume(os.Args[2:])
	case "verify-chain":
		runVerifyChain(os.Args[2:])
	case "uninstall":
		runUninstall(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("BANDIQ Node Agent v%s\n", config.DefaultSoftwareVer)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`BANDIQ DePIN Bandwidth Node Agent

Usage:
  bandiq <command> [arguments]

Commands:
  init          Initialize node identity keypair and associate operator wallet
  consent       Manage explicit operator consent (grant, revoke, status)
  start         Start the background bandwidth node agent daemon
  test          Run a single on-demand bandwidth benchmark and display results
  status        Check current health and performance metrics of running agent
  pause         Pause active bandwidth testing on running agent
  resume        Resume active bandwidth testing on running agent
  verify-chain  Verify Sidra Chain RPC connectivity and query BNDQ token balance
  uninstall     Cleanly remove agent data, keys, and spooled reports

Use "bandiq <command> --help" for more information about a command.
`)
}

func setupLogger(levelStr string) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

func resolveDataDir(custom string) string {
	if custom != "" {
		return custom
	}
	if envDir := os.Getenv("BANDIQ_DATA_DIR"); envDir != "" {
		return envDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".bandiq"
	}
	return filepath.Join(home, ".bandiq")
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	wallet := fs.String("wallet", "", "Operator EVM wallet address on Sidra Chain (0x...)")
	dataDirFlag := fs.String("data-dir", "", "Custom data directory (default: ~/.bandiq)")
	_ = fs.Parse(args)

	dataDir := resolveDataDir(*dataDirFlag)
	keyPath := filepath.Join(dataDir, "node_key.json")

	if *wallet == "" {
		*wallet = os.Getenv("BANDIQ_OPERATOR_WALLET")
	}

	if *wallet == "" {
		fmt.Print("Enter operator EVM wallet address (0x...): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		*wallet = strings.TrimSpace(input)
	}

	if err := identity.ValidateWallet(*wallet); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if existing, err := identity.LoadFromFile(keyPath); err == nil {
		fmt.Println("[Init] Existing node identity detected:")
		fmt.Printf("  Node ID:         %s\n", existing.NodeID)
		fmt.Printf("  Public Key:      %s\n", existing.PublicKeyHex())
		fmt.Printf("  Operator Wallet: %s\n", existing.OperatorWallet)
		fmt.Printf("  Key File:        %s\n", keyPath)
		return
	}

	id, err := identity.GenerateNewIdentity(*wallet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate identity: %v\n", err)
		os.Exit(1)
	}

	if err := id.SaveToFile(keyPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save key file: %v\n", err)
		os.Exit(1)
	}

	cfg := config.DefaultConfig(dataDir)
	cfg.Operator.WalletAddress = *wallet
	_ = cfg.SaveToFile(filepath.Join(dataDir, "config.json"))

	fmt.Println("[Init] Node successfully initialized!")
	fmt.Printf("  Node ID:         %s\n", id.NodeID)
	fmt.Printf("  Public Key:      %s\n", id.PublicKeyHex())
	fmt.Printf("  Operator Wallet: %s\n", id.OperatorWallet)
	fmt.Printf("  Key Store:       %s\n", keyPath)
	fmt.Println("\nNext step: Run 'bandiq consent grant' to authorize measurement telemetry.")
}

func runConsent(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: bandiq consent [grant | revoke | status]")
		os.Exit(1)
	}

	sub := args[0]
	dataDir := resolveDataDir("")
	consentMgr, err := consent.NewManager(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize consent manager: %v\n", err)
		os.Exit(1)
	}

	keyPath := filepath.Join(dataDir, "node_key.json")
	id, _ := identity.LoadFromFile(keyPath)

	switch sub {
	case "grant":
		wallet := ""
		if id != nil {
			wallet = id.OperatorWallet
		}
		nodeID := ""
		if id != nil {
			nodeID = id.NodeID
		}
		if err := consentMgr.Grant(wallet, nodeID, true, false); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to grant consent: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[Consent] Explicit operator consent GRANTED.")
		fmt.Println("  Active bandwidth tests: ENABLED")
		fmt.Println("  Traffic relay / proxy:  DISABLED (Strictly isolated)")
		fmt.Printf("  Operator: %s\n", wallet)

	case "revoke":
		if err := consentMgr.Revoke(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to revoke consent: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[Consent] Operator consent REVOKED. All measurement and relay disabled.")

	case "status":
		st := consentMgr.Status()
		fmt.Println("[Consent Status]")
		fmt.Printf("  Granted:           %v\n", st.Granted)
		if st.Granted {
			fmt.Printf("  Granted At:        %s\n", st.GrantedAt.Format(time.RFC3339))
			fmt.Printf("  Operator:          %s\n", st.OperatorWallet)
			fmt.Printf("  Allow Active Test: %v\n", st.AllowActiveTests)
			fmt.Printf("  Allow Relay:       %v\n", st.AllowTrafficRelay)
		}
	default:
		fmt.Printf("Unknown consent subcommand: %s\n", sub)
	}
}

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to config file (default: ~/.bandiq/config.json)")
	dataDirFlag := fs.String("data-dir", "", "Custom data directory")
	_ = fs.Parse(args)

	dataDir := resolveDataDir(*dataDirFlag)
	if *configPath == "" {
		*configPath = filepath.Join(dataDir, "config.json")
	}

	cfg, err := config.LoadFromFile(*configPath)
	if err != nil {
		cfg = config.DefaultConfig(dataDir)
	}

	logger := setupLogger(cfg.LogLevel)

	fmt.Printf(banner, cfg.SoftwareVersion, cfg.Network.TargetChainID, cfg.Network.TokenContractAddress)

	// 1. Identity Verification
	keyPath := filepath.Join(dataDir, "node_key.json")
	id, err := identity.LoadFromFile(keyPath)
	if err != nil {
		logger.Error("Node identity not found. Please run 'bandiq init --wallet <address>' first.", "path", keyPath)
		os.Exit(1)
	}

	logger.Info("Node identity loaded",
		"node_id", id.NodeID,
		"wallet", id.OperatorWallet,
		"version", cfg.SoftwareVersion)

	// 2. Consent Check
	consentMgr, err := consent.NewManager(dataDir)
	if err != nil {
		logger.Error("Failed to initialize consent manager", "error", err)
		os.Exit(1)
	}

	if !consentMgr.HasConsent() {
		logger.Warn("ATTENTION: Operator consent has not been granted! Network tests are paused. Run 'bandiq consent grant' to activate.")
	}

	// 3. Persistent Spool Queue
	spoolQueue, err := spool.NewQueue(cfg.Backend.SpoolDir, cfg.Backend.MaxSpoolReports)
	if err != nil {
		logger.Error("Failed to initialize report spool queue", "error", err)
		os.Exit(1)
	}
	pendingSpool, _ := spoolQueue.Count()
	logger.Info("Report spool queue initialized", "spool_dir", cfg.Backend.SpoolDir, "pending_reports", pendingSpool)

	// 4. System Telemetry Collector
	collector := system.NewCollector()

	// 5. Sidra Chain Verification
	chainClient := blockchain.NewClient(cfg.Network.RPCURL, cfg.Network.TokenContractAddress)
	chainStatus := "VERIFIED"
	if cfg.Network.VerifyChainOnStartup {
		ctxRPC, cancelRPC := context.WithTimeout(context.Background(), 5*time.Second)
		chainID, err := chainClient.VerifyChainID(ctxRPC)
		cancelRPC()
		if err != nil {
			logger.Warn("Sidra Chain RPC unreachable or verification skipped (operating in simulated verification mode)",
				"rpc_url", cfg.Network.RPCURL, "error", err)
			chainStatus = "SIMULATED_OFFLINE"
		} else {
			logger.Info("Connected to Sidra Chain", "chain_id", chainID)
		}
	}

	// 6. Backend Client
	backendClient := client.NewClient(cfg.Backend.BaseURL, id, spoolQueue)
	ctxReg, cancelReg := context.WithTimeout(context.Background(), 5*time.Second)
	if err := backendClient.RegisterNode(ctxReg, cfg.SoftwareVersion); err != nil {
		logger.Warn("Backend registration pending (will retry during report submissions)", "backend", cfg.Backend.BaseURL, "error", err)
	} else {
		logger.Info("Registered node with BANDIQ backend", "backend", cfg.Backend.BaseURL)
	}
	cancelReg()

	// 7. Measurement Engine
	measureEngine := measure.NewEngine()
	var sequenceCounter uint64 = 0

	// Benchmark trigger helper function
	executeTest := func() (*report.MeasurementReport, error) {
		if !consentMgr.CanRunActiveTests() {
			return nil, consent.ErrConsentNotGranted
		}
		if len(cfg.Measurement.TestServers) == 0 {
			return nil, measure.ErrNoServerAvailable
		}
		serverURL := cfg.Measurement.TestServers[0]

		logger.Info("Executing network benchmark", "server", serverURL)
		metrics, err := measureEngine.RunBenchmark(context.Background(), serverURL, &cfg.Measurement, consentMgr)
		if err != nil {
			return nil, err
		}

		sysMetrics, _ := collector.GetSystemMetrics()
		ifaceCounters, _ := collector.GetPrimaryInterfaceCounters()

		sequenceCounter++
		rep := &report.MeasurementReport{
			ReportID:          report.GenerateReportID(),
			SequenceNumber:    sequenceCounter,
			Timestamp:         time.Now().UTC(),
			SoftwareVersion:   cfg.SoftwareVersion,
			TestServerID:      serverURL,
			MeasurementMethod: "HTTP_SPEEDTEST_V1",
			Metrics:           *metrics,
			InterfaceCounters: ifaceCounters,
			SystemMetrics:     sysMetrics,
		}

		if err := rep.Sign(id); err != nil {
			return nil, fmt.Errorf("failed to sign report: %w", err)
		}

		logger.Info("Benchmark completed and cryptographically signed",
			"report_id", rep.ReportID,
			"seq", rep.SequenceNumber,
			"download_mbps", rep.Metrics.DownloadMbps,
			"upload_mbps", rep.Metrics.UploadMbps,
			"latency_ms", rep.Metrics.LatencyMs,
			"active_bytes_in", rep.Metrics.ActiveTestBytesIn,
			"served_bytes", rep.Metrics.ServedBytesIn)

		return rep, nil
	}

	// 8. Admin Server
	adminServer, err := admin.NewServer(cfg, consentMgr, spoolQueue, collector, id.NodeID, id.OperatorWallet, executeTest)
	if err != nil {
		logger.Error("Failed to initialize admin server", "error", err)
		os.Exit(1)
	}
	adminServer.SetChainStatus(chainStatus)

	if err := adminServer.Start(); err != nil {
		logger.Error("Failed to bind admin server", "bind", cfg.Admin.BindAddress, "error", err)
		os.Exit(1)
	}
	logger.Info("Local administrative server active", "bind", cfg.Admin.BindAddress)

	// 9. Main Worker Loop
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(cfg.TestInterval())
	defer ticker.Stop()

	spoolTicker := time.NewTicker(time.Duration(cfg.Backend.ReportIntervalSec) * time.Second)
	defer spoolTicker.Stop()

	// Run initial test on startup if consent given
	if consentMgr.CanRunActiveTests() && cfg.Measurement.Enabled {
		if rep, err := executeTest(); err == nil {
			adminServer.RecordCompletedTest(rep)
			_ = backendClient.SubmitReport(ctx, rep)
		} else {
			logger.Warn("Initial benchmark skipped or failed", "error", err)
		}
	}

	logger.Info("BANDIQ Node Agent running. Press Ctrl+C to stop.")

	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutdown signal received, gracefully terminating...")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = adminServer.Stop(shutdownCtx)
			cancel()
			logger.Info("Agent safely stopped.")
			return

		case <-spoolTicker.C:
			// Drain offline spool queue if any items pending
			if count, _ := spoolQueue.Count(); count > 0 {
				flushed, err := backendClient.DrainSpool(ctx, 5)
				if err == nil && flushed > 0 {
					logger.Info("Flushed pending reports from offline spool", "flushed", flushed)
				}
			}

		case <-ticker.C:
			if adminServer.IsPaused() {
				logger.Debug("Measurement tick skipped: agent is paused")
				continue
			}
			if !consentMgr.CanRunActiveTests() {
				logger.Debug("Measurement tick skipped: operator consent missing")
				continue
			}

			rep, err := executeTest()
			if err != nil {
				logger.Warn("Scheduled measurement failed", "error", err)
				continue
			}

			adminServer.RecordCompletedTest(rep)

			if cfg.Backend.SubmitReports {
				submitCtx, submitCancel := context.WithTimeout(ctx, 10*time.Second)
				if err := backendClient.SubmitReport(submitCtx, rep); err != nil {
					logger.Warn("Direct report submission failed (saved to offline spool)", "error", err)
				} else {
					logger.Info("Report successfully delivered to backend", "report_id", rep.ReportID)
				}
				submitCancel()
			}
		}
	}
}

func runTest(args []string) {
	dataDir := resolveDataDir("")
	keyPath := filepath.Join(dataDir, "node_key.json")
	id, err := identity.LoadFromFile(keyPath)
	if err != nil {
		fmt.Println("Node not initialized. Running in ephemeral test mode with generated key.")
		id, _ = identity.GenerateNewIdentity("0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133")
	}

	consentMgr, _ := consent.NewManager(dataDir)
	if !consentMgr.HasConsent() {
		_ = consentMgr.Grant(id.OperatorWallet, id.NodeID, true, false)
	}

	cfg := config.DefaultConfig(dataDir)
	serverURL := cfg.Measurement.TestServers[0]
	if len(args) > 0 {
		serverURL = args[0]
	}

	fmt.Printf("[BANDIQ Speedtest] Target server: %s\n", serverURL)
	engine := measure.NewEngine()

	metrics, err := engine.RunBenchmark(context.Background(), serverURL, &cfg.Measurement, consentMgr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Benchmark failed: %v\n", err)
		os.Exit(1)
	}

	collector := system.NewCollector()
	sysMetrics, _ := collector.GetSystemMetrics()
	ifaceCounters, _ := collector.GetPrimaryInterfaceCounters()

	rep := &report.MeasurementReport{
		ReportID:          report.GenerateReportID(),
		SequenceNumber:    1,
		Timestamp:         time.Now().UTC(),
		SoftwareVersion:   cfg.SoftwareVersion,
		TestServerID:      serverURL,
		MeasurementMethod: "HTTP_SPEEDTEST_V1",
		Metrics:           *metrics,
		InterfaceCounters: ifaceCounters,
		SystemMetrics:     sysMetrics,
	}
	_ = rep.Sign(id)

	fmt.Println("\n================ Benchmark Results ================")
	fmt.Printf("  Download Throughput:   %.2f Mbps\n", metrics.DownloadMbps)
	fmt.Printf("  Upload Throughput:     %.2f Mbps\n", metrics.UploadMbps)
	fmt.Printf("  Latency (RTT avg):     %.2f ms (min: %.2f ms, max: %.2f ms)\n", metrics.LatencyMs, metrics.MinLatencyMs, metrics.MaxLatencyMs)
	fmt.Printf("  Statistical Jitter:    %.2f ms\n", metrics.JitterMs)
	fmt.Printf("  Packet Loss Estimate:  %.1f %%\n", metrics.PacketLossPct)
	fmt.Printf("  Synthetic Test Bytes:  In: %d bytes | Out: %d bytes\n", metrics.ActiveTestBytesIn, metrics.ActiveTestBytesOut)
	fmt.Printf("  Served Relay Bytes:    In: %d bytes | Out: %d bytes\n", metrics.ServedBytesIn, metrics.ServedBytesOut)
	fmt.Println("====================================================")
	fmt.Printf("  Report ID:             %s\n", rep.ReportID)
	fmt.Printf("  Node ID:               %s\n", rep.NodeID)
	fmt.Printf("  Signature:             %s\n", rep.Signature[:24]+"...")
	fmt.Println("====================================================")
}

func runStatus(args []string) {
	resp, err := http.Get("http://127.0.0.1:8725/status")
	if err != nil {
		fmt.Println("[Status] BANDIQ agent daemon is NOT running on localhost:8725.")
		dataDir := resolveDataDir("")
		keyPath := filepath.Join(dataDir, "node_key.json")
		if id, err := identity.LoadFromFile(keyPath); err == nil {
			fmt.Printf("  Configured Node ID: %s\n", id.NodeID)
			fmt.Printf("  Operator Wallet:    %s\n", id.OperatorWallet)
		}
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var out bytes.Buffer
	_ = json.Indent(&out, body, "", "  ")
	fmt.Println(out.String())
}

func runPause(args []string) {
	resp, err := http.Post("http://127.0.0.1:8725/pause", "application/json", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to agent: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	fmt.Println("[Control] Agent measurements PAUSED.")
}

func runResume(args []string) {
	resp, err := http.Post("http://127.0.0.1:8725/resume", "application/json", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to agent: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	fmt.Println("[Control] Agent measurements RESUMED.")
}

func runVerifyChain(args []string) {
	dataDir := resolveDataDir("")
	configPath := filepath.Join(dataDir, "config.json")
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		cfg = config.DefaultConfig(dataDir)
		cfg.ApplyEnvOverrides()
	}

	keyPath := filepath.Join(dataDir, "node_key.json")
	id, _ := identity.LoadFromFile(keyPath)
	wallet := cfg.Operator.WalletAddress
	if id != nil {
		wallet = id.OperatorWallet
	}

	fmt.Printf("[Sidra Chain Verification]\n")
	fmt.Printf("  Expected Chain ID: %d\n", cfg.Network.TargetChainID)
	fmt.Printf("  Token Contract:    %s ($BNDQ)\n", cfg.Network.TokenContractAddress)
	fmt.Printf("  Operator Wallet:   %s\n", wallet)
	fmt.Printf("  RPC Endpoint:      %s\n\n", cfg.Network.RPCURL)

	client := blockchain.NewClient(cfg.Network.RPCURL, cfg.Network.TokenContractAddress)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	chainID, err := client.VerifyChainID(ctx)
	if err != nil {
		fmt.Printf("  RPC Check: FAILED (%v)\n", err)
		fmt.Println("  Note: Sidra public RPC might be experiencing maintenance or latency.")
		fmt.Println("  You can run the mock backend with: .\\scripts\\run-mock-backend.ps1")
		return
	}

	fmt.Printf("  RPC Check: SUCCESS (Chain ID confirmed: %d)\n", chainID)

	if wallet != "" {
		bal, err := client.GetBNDQBalance(ctx, wallet)
		if err != nil {
			fmt.Printf("  Token Balance Check: FAILED (%v)\n", err)
		} else {
			fmt.Printf("  BNDQ Raw Balance:    %s\n", bal.String())
		}
	}
}

func runUninstall(args []string) {
	dataDir := resolveDataDir("")
	fmt.Println("=====================================================")
	fmt.Println("           BANDIQ Agent Clean Uninstaller           ")
	fmt.Println("=====================================================")
	fmt.Printf("This will completely remove the BANDIQ data directory:\n  %s\n\n", dataDir)
	fmt.Println("Includes: node keys, operator config, consent, and spooled reports.")
	fmt.Println("Note: This will NOT modify firewall, router, or startup settings.")
	fmt.Print("\nAre you sure you want to proceed? (type 'yes' to confirm): ")

	reader := bufio.NewReader(os.Stdin)
	ans, _ := reader.ReadString('\n')
	ans = strings.TrimSpace(strings.ToLower(ans))

	if ans != "yes" {
		fmt.Println("Uninstallation aborted.")
		return
	}

	if err := os.RemoveAll(dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to remove data directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[Uninstall] Data directory removed successfully.")
	fmt.Println("To complete removal, delete the executable file.")
}

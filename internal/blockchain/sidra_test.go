package blockchain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSidraChainVerification(t *testing.T) {
	// Mock RPC server returning Sidra Chain ID 97453 (0x17cb5)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		if req.Method == "eth_chainId" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  "0x17cad", // 97453 in hex
			})
			return
		}

		if req.Method == "eth_call" {
			// Mock 1,000,000 BNDQ (18 decimals: 1000000 * 10^18 = 0xd3c21bcecceda1000000)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  "0x00000000000000000000000000000000000000000000d3c21bcecceda1000000",
			})
			return
		}

		http.Error(w, "unknown method", http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, ExpectedTokenAddress)

	chainID, err := client.VerifyChainID(context.Background())
	if err != nil {
		t.Fatalf("chain verification failed: %v", err)
	}
	if chainID != 97453 {
		t.Fatalf("expected chain ID 97453, got %d", chainID)
	}

	bal, err := client.GetBNDQBalance(context.Background(), "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133")
	if err != nil {
		t.Fatalf("failed to get BNDQ balance: %v", err)
	}
	if bal.Sign() <= 0 {
		t.Fatalf("expected positive balance")
	}
}

func TestSidraChainMismatch(t *testing.T) {
	// Mock server returning Ethereum Mainnet (1 = 0x1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  "0x1",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, ExpectedTokenAddress)
	_, err := client.VerifyChainID(context.Background())
	if err == nil {
		t.Fatalf("expected chain mismatch error, got nil")
	}
}

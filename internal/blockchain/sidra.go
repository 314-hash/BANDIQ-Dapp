package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bandiq-node-agent/internal/identity"
)

const (
	ExpectedChainID      uint64 = 97453
	ExpectedTokenAddress        = "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133"
)

var (
	ErrChainMismatch     = errors.New("connected RPC returned unexpected chain ID (mismatch with Sidra Chain 97453)")
	ErrRPCUnavailable    = errors.New("sidra chain RPC endpoint unavailable")
	ErrInvalidRPCResponse = errors.New("invalid JSON-RPC response from blockchain node")
)

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client interacts with Sidra Chain via standard EVM JSON-RPC.
type Client struct {
	rpcURL       string
	tokenAddress string
	httpClient   *http.Client
}

// NewClient creates a new Sidra Chain client.
func NewClient(rpcURL, tokenAddress string) *Client {
	if rpcURL == "" {
		rpcURL = "https://rpc.sidrachain.com"
	}
	if tokenAddress == "" {
		tokenAddress = ExpectedTokenAddress
	}

	return &Client{
		rpcURL:       rpcURL,
		tokenAddress: tokenAddress,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Call executes an arbitrary JSON-RPC call.
func (c *Client) Call(ctx context.Context, method string, params ...interface{}) (json.RawMessage, error) {
	reqBody := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRPCUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rpc HTTP status %d: %s", resp.StatusCode, string(body))
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRPCResponse, err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// VerifyChainID queries eth_chainId and validates it against Sidra Chain (97453).
func (c *Client) VerifyChainID(ctx context.Context) (uint64, error) {
	raw, err := c.Call(ctx, "eth_chainId")
	if err != nil {
		return 0, err
	}

	var hexStr string
	if err := json.Unmarshal(raw, &hexStr); err != nil {
		return 0, fmt.Errorf("failed to parse chainId result: %w", err)
	}

	cleanHex := strings.TrimPrefix(hexStr, "0x")
	chainID, err := strconv.ParseUint(cleanHex, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse hex chainId '%s': %w", hexStr, err)
	}

	if chainID != ExpectedChainID {
		return chainID, fmt.Errorf("%w: got %d, expected %d", ErrChainMismatch, chainID, ExpectedChainID)
	}

	return chainID, nil
}

// GetBNDQBalance queries the ERC-20 BNDQ token contract for the operator's balance.
func (c *Client) GetBNDQBalance(ctx context.Context, walletAddress string) (*big.Int, error) {
	if err := identity.ValidateWallet(walletAddress); err != nil {
		return nil, err
	}

	// ERC-20 balanceOf(address) selector: 0x70a08231
	cleanWallet := strings.ToLower(strings.TrimPrefix(walletAddress, "0x"))
	paddedWallet := fmt.Sprintf("%064s", cleanWallet)
	callData := "0x70a08231" + paddedWallet

	callObj := map[string]string{
		"to":   c.tokenAddress,
		"data": callData,
	}

	raw, err := c.Call(ctx, "eth_call", callObj, "latest")
	if err != nil {
		return nil, err
	}

	var hexResult string
	if err := json.Unmarshal(raw, &hexResult); err != nil {
		return nil, fmt.Errorf("failed to parse eth_call result: %w", err)
	}

	cleanResult := strings.TrimPrefix(hexResult, "0x")
	if cleanResult == "" {
		return big.NewInt(0), nil
	}

	balance := new(big.Int)
	balance.SetString(cleanResult, 16)
	return balance, nil
}

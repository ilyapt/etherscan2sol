package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// Client is a generic JSON-RPC client.
type Client struct {
	rpcURL     string
	httpClient *http.Client
}

// NewClient creates a new JSON-RPC client.
func NewClient(rpcURL string) *Client {
	return &Client{
		rpcURL: rpcURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Call performs a JSON-RPC call and returns the raw result.
func (c *Client) Call(method string, params any) (json.RawMessage, error) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", method, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("POST %s: HTTP %d", method, resp.StatusCode)
	}

	var rpcResp jsonRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode response for %s: %w", method, err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// GetStorageAt reads a 32-byte storage slot from a contract.
func (c *Client) GetStorageAt(address string, slot *big.Int, block string) ([32]byte, error) {
	slotHex := fmt.Sprintf("0x%064x", slot)
	result, err := c.Call("eth_getStorageAt", []string{address, slotHex, block})
	if err != nil {
		return [32]byte{}, fmt.Errorf("eth_getStorageAt: %w", err)
	}

	var hexStr string
	if err := json.Unmarshal(result, &hexStr); err != nil {
		return [32]byte{}, fmt.Errorf("parse storage result: %w", err)
	}

	hexStr = strings.TrimPrefix(hexStr, "0x")
	// Pad to 64 hex chars (32 bytes)
	for len(hexStr) < 64 {
		hexStr = "0" + hexStr
	}

	var data [32]byte
	for i := 0; i < 32; i++ {
		fmt.Sscanf(hexStr[i*2:i*2+2], "%02x", &data[i])
	}

	return data, nil
}

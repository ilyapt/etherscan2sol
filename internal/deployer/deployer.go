package deployer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type Deployer struct {
	rpcURL     string
	httpClient *http.Client
}

func NewDeployer(rpcURL string) *Deployer {
	return &Deployer{
		rpcURL: rpcURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (d *Deployer) call(method string, params interface{}) (json.RawMessage, error) {
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

	resp, err := d.httpClient.Post(d.rpcURL, "application/json", bytes.NewReader(body))
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

func (d *Deployer) Deploy(bytecode string) (string, error) {
	// Step 1: Get accounts
	fmt.Fprintf(os.Stderr, "Fetching accounts from %s...\n", d.rpcURL)
	result, err := d.call("eth_accounts", []interface{}{})
	if err != nil {
		return "", fmt.Errorf("eth_accounts: %w", err)
	}

	var accounts []string
	if err := json.Unmarshal(result, &accounts); err != nil {
		return "", fmt.Errorf("parse accounts: %w", err)
	}
	if len(accounts) == 0 {
		return "", fmt.Errorf("no accounts available on local node")
	}
	fmt.Fprintf(os.Stderr, "Using account: %s\n", accounts[0])

	// Step 2: Send deployment transaction
	fmt.Fprintf(os.Stderr, "Sending deployment transaction...\n")
	result, err = d.call("eth_sendTransaction", []interface{}{map[string]string{
		"from": accounts[0],
		"data": "0x" + bytecode,
		"gas":  "0x1000000",
	}})
	if err != nil {
		return "", fmt.Errorf("eth_sendTransaction: %w", err)
	}

	var txHash string
	if err := json.Unmarshal(result, &txHash); err != nil {
		return "", fmt.Errorf("parse transaction hash: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Transaction hash: %s\n", txHash)

	// Step 3: Poll for receipt
	var receipt transactionReceipt
	const maxAttempts = 30
	for i := 0; i < maxAttempts; i++ {
		fmt.Fprintf(os.Stderr, "Polling for receipt (attempt %d/%d)...\n", i+1, maxAttempts)
		result, err = d.call("eth_getTransactionReceipt", []interface{}{txHash})
		if err != nil {
			return "", fmt.Errorf("eth_getTransactionReceipt: %w", err)
		}

		if string(result) != "null" && len(result) > 0 {
			if err := json.Unmarshal(result, &receipt); err != nil {
				return "", fmt.Errorf("parse receipt: %w", err)
			}
			break
		}

		if i == maxAttempts-1 {
			return "", fmt.Errorf("transaction receipt not available after %d attempts", maxAttempts)
		}
		time.Sleep(1 * time.Second)
	}

	// Step 4: Check status
	if receipt.Status != "0x1" {
		return "", fmt.Errorf("transaction failed with status: %s", receipt.Status)
	}

	fmt.Fprintf(os.Stderr, "Contract deployed at: %s\n", receipt.ContractAddress)
	return receipt.ContractAddress, nil
}

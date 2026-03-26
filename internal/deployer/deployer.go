package deployer

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ilyapt/etherscan2sol/internal/rpc"
)

type Deployer struct {
	client *rpc.Client
}

func NewDeployer(rpcURL string) *Deployer {
	return &Deployer{
		client: rpc.NewClient(rpcURL),
	}
}

func (d *Deployer) Deploy(bytecode string) (string, error) {
	// Step 1: Get accounts
	fmt.Fprintf(os.Stderr, "Fetching accounts...\n")
	result, err := d.client.Call("eth_accounts", []any{})
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
	result, err = d.client.Call("eth_sendTransaction", []any{map[string]string{
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
	for i := range maxAttempts {
		fmt.Fprintf(os.Stderr, "Polling for receipt (attempt %d/%d)...\n", i+1, maxAttempts)
		result, err = d.client.Call("eth_getTransactionReceipt", []any{txHash})
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

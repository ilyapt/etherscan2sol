package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Known verified contracts on Ethereum mainnet (chain 1) that won't change.
// Using contracts compiled with solc 0.8.x to ensure compiler availability.
var snapshotCases = []struct {
	name     string
	command  string
	chainID  string
	address  string
	wantHash string // sha256 of stdout; empty = print hash and skip comparison
}{
	{
		name:    "compile_USDT",
		command: "compile",
		chainID: "1",
		address: "0xdAC17F958D2ee523a2206206994597C13D831ec7", // TetherToken, solc 0.4.18
	},
	{
		name:    "storage_USDT",
		command: "storage",
		chainID: "1",
		address: "0xdAC17F958D2ee523a2206206994597C13D831ec7",
	},
	{
		name:     "compile_1inch_v6",
		command:  "compile",
		chainID:  "1",
		address:  "0x111111125421cA6dc452d289314280a0f8842A65", // AggregationRouterV6, solc 0.8.23
		wantHash: "c1cf8309363435d5ff2afa33706214c58c94de019fa6a1430c1629b2f7b54bef",
	},
	{
		name:     "compile_uniswap_universal_router",
		command:  "compile",
		chainID:  "1",
		address:  "0x3fC91A3afd70395Cd496C647d5a6CC9D4B2b7FAD", // UniversalRouter, solc 0.8.17
		wantHash: "cdea5f1c64493b92ff8db5c9e2c813d83087bdf50dc799d4f67359f48d84c1dc",
	},
	{
		name:     "storage_1inch_v6",
		command:  "storage",
		chainID:  "1",
		address:  "0x111111125421cA6dc452d289314280a0f8842A65",
		wantHash: "1f580a0d063b4a812a6ad6c5559a066f4904571cac43c06ccaf85c858761bd4e",
	},
	{
		name:     "storage_uniswap_universal_router",
		command:  "storage",
		chainID:  "1",
		address:  "0x3fC91A3afd70395Cd496C647d5a6CC9D4B2b7FAD",
		wantHash: "2ada2d36356a106e69879341dfb9799912721b3299da49c8b5dfca3de4c37bc1",
	},
}

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once before all tests.
	bin, err := os.CreateTemp("", "etherscan2sol-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	bin.Close()
	binaryPath = bin.Name()

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build binary: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	os.Remove(binaryPath)
	os.Exit(code)
}

func TestSnapshots(t *testing.T) {
	apiKey := os.Getenv("ETHERSCAN_API_KEY")
	if apiKey == "" {
		t.Skip("ETHERSCAN_API_KEY not set, skipping snapshot tests")
	}

	for _, tc := range snapshotCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tc.command, "--api-key", apiKey, tc.chainID, tc.address)
			out, err := cmd.Output()
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					t.Fatalf("command failed (exit %d):\n%s", ee.ExitCode(), string(ee.Stderr))
				}
				t.Fatalf("command failed: %v", err)
			}

			stdout := strings.TrimSpace(string(out))
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(stdout)))

			if tc.wantHash == "" {
				t.Logf("HASH for %s: %s", tc.name, hash)
				t.Logf("output length: %d bytes", len(stdout))
				t.Skip("no expected hash set — run once to capture, then fill in wantHash")
			}

			if hash != tc.wantHash {
				t.Errorf("hash mismatch for %s\n  got:  %s\n  want: %s\n  output length: %d bytes",
					tc.name, hash, tc.wantHash, len(stdout))
			}
		})
	}
}

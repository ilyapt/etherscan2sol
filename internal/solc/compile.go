package solc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Compile runs solc --standard-json and returns the hex bytecode for the given contract.
func Compile(solcPath string, input *StandardJSONInput, contractName string) (string, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshaling standard JSON input: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, solcPath, "--standard-json")
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running solc: %w\nstderr: %s", err, stderr.String())
	}

	var output StandardJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return "", fmt.Errorf("parsing solc output: %w", err)
	}

	// Check for compilation errors.
	var errs []string
	for _, e := range output.Errors {
		if e.Severity == "error" {
			errs = append(errs, e.FormattedMessage)
		}
	}
	if len(errs) > 0 {
		return "", fmt.Errorf("solc compilation errors:\n%s", strings.Join(errs, "\n"))
	}

	// Find the bytecode for the requested contract.
	bytecode := findBytecode(output.Contracts, contractName)
	if bytecode == "" {
		return "", fmt.Errorf("bytecode not found for contract %q", contractName)
	}

	// Check for unlinked library placeholders (__$...$__).
	if strings.Contains(bytecode, "__$") {
		return "", fmt.Errorf("bytecode contains unlinked library references; linking is not supported")
	}

	return bytecode, nil
}

// findBytecode searches the contracts map for the given contract name and returns its bytecode.
func findBytecode(contracts map[string]map[string]ContractOutput, contractName string) string {
	for _, fileContracts := range contracts {
		if co, ok := fileContracts[contractName]; ok {
			if co.EVM.Bytecode.Object != "" {
				return co.EVM.Bytecode.Object
			}
		}
	}
	return ""
}

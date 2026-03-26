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

// CompileResult holds the compilation output for a contract.
type CompileResult struct {
	Bytecode      string
	StorageLayout StorageLayout
}

// Compile runs solc --standard-json and returns the bytecode and storage layout for the given contract.
func Compile(solcPath string, input *StandardJSONInput, contractName string) (*CompileResult, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshaling standard JSON input: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, solcPath, "--standard-json")
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running solc: %w\nstderr: %s", err, stderr.String())
	}

	var output StandardJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("parsing solc output: %w", err)
	}

	// Check for compilation errors.
	var errs []string
	for _, e := range output.Errors {
		if e.Severity == "error" {
			errs = append(errs, e.FormattedMessage)
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("solc compilation errors:\n%s", strings.Join(errs, "\n"))
	}

	// Find the contract output for the requested contract.
	co := findContract(output.Contracts, contractName)
	if co == nil {
		return nil, fmt.Errorf("contract %q not found in compilation output", contractName)
	}

	bytecode := co.EVM.Bytecode.Object
	if bytecode == "" {
		return nil, fmt.Errorf("bytecode not found for contract %q", contractName)
	}

	// Check for unlinked library placeholders (__$...$__).
	if strings.Contains(bytecode, "__$") {
		return nil, fmt.Errorf("bytecode contains unlinked library references; linking is not supported")
	}

	return &CompileResult{
		Bytecode:      bytecode,
		StorageLayout: co.StorageLayout,
	}, nil
}

// findContract searches the contracts map for the given contract name.
func findContract(contracts map[string]map[string]ContractOutput, contractName string) *ContractOutput {
	for _, fileContracts := range contracts {
		if co, ok := fileContracts[contractName]; ok {
			return &co
		}
	}
	return nil
}

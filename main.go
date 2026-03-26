package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ilyapt/etherscan2sol/internal/deployer"
	"github.com/ilyapt/etherscan2sol/internal/etherscan"
	"github.com/ilyapt/etherscan2sol/internal/solc"
)

const usage = `Usage: etherscan2sol <command> [flags] <chainId> <address>

Commands:
  compile    Compile contract and output bytecode
  storage    Compile contract and output storage layout

Global flags:
  --api-key  Etherscan API key (or set ETHERSCAN_API_KEY env var)

Command flags:
  compile --deploy-to=<url>  Deploy bytecode to a node after compilation`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	command := os.Args[1]
	if command != "compile" && command != "storage" {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n%s\n", command, usage)
		os.Exit(1)
	}

	fs := flag.NewFlagSet(command, flag.ExitOnError)
	apiKey := fs.String("api-key", "", "Etherscan API key")
	var deployTo *string
	if command == "compile" {
		deployTo = fs.String("deploy-to", "", "Deploy to this JSON-RPC endpoint")
	}
	fs.Parse(os.Args[2:])

	if *apiKey == "" {
		*apiKey = os.Getenv("ETHERSCAN_API_KEY")
	}
	if *apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: --api-key flag or ETHERSCAN_API_KEY env var is required")
		os.Exit(1)
	}

	args := fs.Args()
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: etherscan2sol %s [flags] <chainId> <address>\n", command)
		os.Exit(1)
	}
	chainID := args[0]
	address := args[1]

	// Fetch and compile
	compiled, contractName := fetchAndCompile(*apiKey, chainID, address)

	switch command {
	case "storage":
		layoutJSON, err := json.MarshalIndent(compiled.StorageLayout, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling storage layout: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(layoutJSON))

	case "compile":
		fmt.Println(compiled.Bytecode)
		fmt.Fprintf(os.Stderr, "Contract: %s, bytecode: %d bytes\n", contractName, len(compiled.Bytecode)/2)

		if deployTo != nil && *deployTo != "" {
			fmt.Fprintf(os.Stderr, "Deploying to %s...\n", *deployTo)
			d := deployer.NewDeployer(*deployTo)
			contractAddr, err := d.Deploy(compiled.Bytecode)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Deployment error: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Deployed at: %s\n", contractAddr)
		}
	}
}

func fetchAndCompile(apiKey, chainID, address string) (*solc.CompileResult, string) {
	client := etherscan.NewClient(apiKey)

	fmt.Fprintf(os.Stderr, "Fetching source code for %s on chain %s...\n", address, chainID)
	result, err := client.GetSourceCode(chainID, address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching source: %v\n", err)
		os.Exit(1)
	}

	if result.Proxy == "1" && result.Implementation != "" {
		fmt.Fprintf(os.Stderr, "Proxy detected, fetching implementation %s...\n", result.Implementation)
		result, err = client.GetSourceCode(chainID, result.Implementation)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching implementation source: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Fprintf(os.Stderr, "Contract: %s, Compiler: %s\n", result.ContractName, result.CompilerVersion)

	input, contractName, err := etherscan.NormalizeSource(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error normalizing source: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Ensuring compiler %s...\n", result.CompilerVersion)
	solcPath, err := solc.EnsureCompiler(result.CompilerVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with compiler: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Compiling %s...\n", contractName)
	compiled, err := solc.Compile(solcPath, input, contractName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
		os.Exit(1)
	}

	return compiled, contractName
}

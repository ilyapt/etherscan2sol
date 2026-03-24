package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ilyapt/etherscan2sol/internal/deployer"
	"github.com/ilyapt/etherscan2sol/internal/etherscan"
	"github.com/ilyapt/etherscan2sol/internal/solc"
)

func main() {
	apiKey := flag.String("api-key", "", "Etherscan API key (or set ETHERSCAN_API_KEY env var)")
	rpcURL := flag.String("rpc-url", "http://localhost:8545", "JSON-RPC endpoint for deployment")
	flag.Parse()

	if *apiKey == "" {
		*apiKey = os.Getenv("ETHERSCAN_API_KEY")
	}
	if *apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: --api-key flag or ETHERSCAN_API_KEY env var is required")
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: etherscan_to_sol [--api-key=KEY] [--rpc-url=URL] <chainId> <address>")
		os.Exit(1)
	}
	chainID := args[0]
	address := args[1]

	// Step 1: Fetch source code from Etherscan
	client := etherscan.NewClient(*apiKey)

	fmt.Fprintf(os.Stderr, "Fetching source code for %s on chain %s...\n", address, chainID)
	result, err := client.GetSourceCode(chainID, address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching source: %v\n", err)
		os.Exit(1)
	}

	// Step 2: If proxy, fetch implementation source
	if result.Proxy == "1" && result.Implementation != "" {
		fmt.Fprintf(os.Stderr, "Proxy detected, fetching implementation %s...\n", result.Implementation)
		result, err = client.GetSourceCode(chainID, result.Implementation)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching implementation source: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Fprintf(os.Stderr, "Contract: %s, Compiler: %s\n", result.ContractName, result.CompilerVersion)

	// Step 3: Normalize source to standard-json input
	input, contractName, err := etherscan.NormalizeSource(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error normalizing source: %v\n", err)
		os.Exit(1)
	}

	// Step 4: Ensure solc compiler is available
	fmt.Fprintf(os.Stderr, "Ensuring compiler %s is available...\n", result.CompilerVersion)
	solcPath, err := solc.EnsureCompiler(result.CompilerVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with compiler: %v\n", err)
		os.Exit(1)
	}

	// Step 5: Compile
	fmt.Fprintf(os.Stderr, "Compiling %s with %s...\n", contractName, solcPath)
	bytecode, err := solc.Compile(solcPath, input, contractName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Compilation successful, bytecode length: %d bytes\n", len(bytecode)/2)

	// Step 6: Deploy to local node
	fmt.Fprintf(os.Stderr, "Deploying to %s...\n", *rpcURL)
	d := deployer.NewDeployer(*rpcURL)
	contractAddr, err := d.Deploy(bytecode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Deployment error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(contractAddr)
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ilyapt/etherscan2sol/internal/deployer"
	"github.com/ilyapt/etherscan2sol/internal/etherscan"
	"github.com/ilyapt/etherscan2sol/internal/rpc"
	"github.com/ilyapt/etherscan2sol/internal/solc"
	"github.com/ilyapt/etherscan2sol/internal/storage"
)

const usage = `Usage: etherscan2sol <command> [flags] <chainId> <address> [args...]

Commands:
  compile    Compile contract and output bytecode
  storage    Compile contract and output storage layout
  read       Read a storage variable value from a deployed contract

Global flags:
  --api-key  Etherscan API key (or set ETHERSCAN_API_KEY env var)

Command flags:
  compile --deploy-to=<url>                Deploy bytecode after compilation
  read    --rpc-url=<url> [--block=latest] RPC endpoint for reading storage`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "compile", "storage", "read":
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n%s\n", command, usage)
		os.Exit(1)
	}

	fs := flag.NewFlagSet(command, flag.ExitOnError)
	apiKey := fs.String("api-key", "", "Etherscan API key")

	var deployTo *string
	if command == "compile" {
		deployTo = fs.String("deploy-to", "", "Deploy to this JSON-RPC endpoint")
	}

	var rpcURL, block *string
	if command == "read" {
		rpcURL = fs.String("rpc-url", "", "JSON-RPC endpoint for reading storage")
		block = fs.String("block", "latest", "Block number or tag")
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

	switch command {
	case "compile", "storage":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: etherscan2sol %s [flags] <chainId> <address>\n", command)
			os.Exit(1)
		}
		runCompileOrStorage(command, *apiKey, args[0], args[1], deployTo)

	case "read":
		if rpcURL == nil || *rpcURL == "" {
			fmt.Fprintln(os.Stderr, "Error: --rpc-url is required for the read command")
			os.Exit(1)
		}
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: etherscan2sol read --rpc-url=<url> [--block=latest] <chainId> <address> <variable> [key1] [key2] ...")
			os.Exit(1)
		}
		chainID, address, variable := args[0], args[1], args[2]
		keys := args[3:]
		runRead(*apiKey, *rpcURL, *block, chainID, address, variable, keys)
	}
}

func runCompileOrStorage(command, apiKey, chainID, address string, deployTo *string) {
	compiled, contractName := fetchAndCompile(apiKey, chainID, address)

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

func runRead(apiKey, rpcURL, block, chainID, address, variable string, keys []string) {
	// Compile to get storageLayout (uses implementation source for proxies)
	compiled, _ := fetchAndCompile(apiKey, chainID, address)

	// Resolve variable + keys to storage slot(s)
	slots, err := storage.Resolve(compiled.StorageLayout, variable, keys)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving variable: %v\n", err)
		os.Exit(1)
	}

	// Read and decode each slot
	client := rpc.NewClient(rpcURL)
	values := make([]*storage.DecodedValue, 0, len(slots))

	for _, slot := range slots {
		raw, err := client.GetStorageAt(address, slot.Slot, block)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading storage: %v\n", err)
			os.Exit(1)
		}

		val, err := storage.Decode(raw, slot, compiled.StorageLayout.Types)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding value: %v\n", err)
			os.Exit(1)
		}
		values = append(values, val)
	}

	// Output
	if len(values) == 1 {
		fmt.Println(values[0].Value)
	} else {
		// Struct: output as JSON object
		obj := make(map[string]any, len(values))
		for _, v := range values {
			obj[v.Label] = v.Value
		}
		out, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling output: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
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

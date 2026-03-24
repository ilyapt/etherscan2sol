# etherscan_to_sol

Downloads verified contract source code via Etherscan v2 API, compiles it with the original compiler and settings, and deploys to a local node. Automatically resolves proxy contracts to their implementation.

## Usage

```
go build -o etherscan_to_sol .

ETHERSCAN_API_KEY=<key> ./etherscan_to_sol <chainId> <address>
```

Outputs the deployed contract address to stdout. Logs progress to stderr.

## Options

```
--api-key    Etherscan API key (overrides ETHERSCAN_API_KEY env var)
--rpc-url    JSON-RPC endpoint (default: http://localhost:8545)
```

## How it works

1. Fetches source code and compiler metadata from Etherscan
2. If the contract is a proxy, follows to the implementation
3. Downloads the matching solc compiler (cached in `~/Library/Caches/etherscan_to_sol/solidity/`)
4. Compiles using original parameters (optimizer, runs, evm version)
5. Deploys bytecode to the local node via `eth_sendTransaction`

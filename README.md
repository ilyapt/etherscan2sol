# etherscan2sol

Downloads verified contract source code via Etherscan v2 API, compiles it with the original compiler and settings. Automatically resolves proxy contracts to their implementation.

## Usage

```
go build -o etherscan2sol .

# Get storage layout
ETHERSCAN_API_KEY=<key> ./etherscan2sol storage <chainId> <address>

# Get deployment bytecode
ETHERSCAN_API_KEY=<key> ./etherscan2sol compile <chainId> <address>

# Compile and deploy to a local node
ETHERSCAN_API_KEY=<key> ./etherscan2sol compile --deploy-to=http://localhost:8545 <chainId> <address>
```

Results go to stdout, progress to stderr.

## Flags

```
--api-key      Etherscan API key (overrides ETHERSCAN_API_KEY env var)
--deploy-to    Deploy compiled bytecode to this JSON-RPC endpoint (compile only)
```

## How it works

1. Fetches source code and compiler metadata from Etherscan v2 API
2. If the contract is a proxy, follows to the implementation
3. Downloads the matching solc compiler (cached in `~/Library/Caches/etherscan2sol/solidity/`)
4. Compiles using original parameters (optimizer, runs, evm version)

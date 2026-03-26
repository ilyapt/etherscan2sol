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

# Read a storage variable
ETHERSCAN_API_KEY=<key> ./etherscan2sol read --rpc-url=https://rpc.soniclabs.com 146 <address> _owner

# Read a mapping value
ETHERSCAN_API_KEY=<key> ./etherscan2sol read --rpc-url=https://rpc.soniclabs.com 146 <address> balances 0xabc...

# Read a nested mapping
ETHERSCAN_API_KEY=<key> ./etherscan2sol read --rpc-url=... 1 <address> allowances 0xowner 0xspender
```

Results go to stdout, progress to stderr.

## Flags

```
--api-key      Etherscan API key (overrides ETHERSCAN_API_KEY env var)
--deploy-to    Deploy compiled bytecode to this JSON-RPC endpoint (compile only)
--rpc-url      JSON-RPC endpoint for reading storage (read only)
--block        Block number or tag, default "latest" (read only)
```

## How it works

1. Fetches source code and compiler metadata from Etherscan v2 API
2. If the contract is a proxy, follows to the implementation
3. Downloads the matching solc compiler (cached locally)
4. Compiles using original parameters (optimizer, runs, evm version)
5. For `read`: resolves variable name to storage slot via storageLayout, reads via `eth_getStorageAt`, decodes the value

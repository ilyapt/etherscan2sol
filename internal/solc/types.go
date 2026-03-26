package solc

import "encoding/json"

// StandardJSONInput is the input format for solc --standard-json.
type StandardJSONInput struct {
	Language string                  `json:"language"`
	Sources  map[string]SourceEntry  `json:"sources"`
	Settings CompilerSettings        `json:"settings"`
}

type SourceEntry struct {
	Content string `json:"content"`
}

type CompilerSettings struct {
	Remappings      []string                             `json:"remappings,omitempty"`
	Optimizer       OptimizerSettings                    `json:"optimizer"`
	EVMVersion      string                               `json:"evmVersion,omitempty"`
	ViaIR           bool                                 `json:"viaIR,omitempty"`
	Metadata        *MetadataSettings                    `json:"metadata,omitempty"`
	OutputSelection map[string]map[string][]string        `json:"outputSelection"`
	Libraries       map[string]map[string]string          `json:"libraries,omitempty"`
}

type MetadataSettings struct {
	UseLiteralContent bool   `json:"useLiteralContent,omitempty"`
	BytecodeHash      string `json:"bytecodeHash,omitempty"`
}

type OptimizerSettings struct {
	Enabled bool `json:"enabled"`
	Runs    int  `json:"runs"`
}

// StandardJSONOutput is the output format from solc --standard-json.
type StandardJSONOutput struct {
	Errors    []CompilerError                          `json:"errors"`
	Contracts map[string]map[string]ContractOutput     `json:"contracts"`
}

type CompilerError struct {
	Severity        string `json:"severity"`
	Type            string `json:"type"`
	Component       string `json:"component"`
	FormattedMessage string `json:"formattedMessage"`
	Message         string `json:"message"`
}

type ContractOutput struct {
	ABI           json.RawMessage `json:"abi"`
	EVM           EVMOutput       `json:"evm"`
	StorageLayout StorageLayout   `json:"storageLayout"`
}

type StorageLayout struct {
	Storage []StorageEntry `json:"storage"`
	Types   map[string]StorageType `json:"types"`
}

type StorageEntry struct {
	ASTId    int    `json:"astId"`
	Contract string `json:"contract"`
	Label    string `json:"label"`
	Offset   int    `json:"offset"`
	Slot     string `json:"slot"`
	Type     string `json:"type"`
}

type StorageType struct {
	Encoding      string `json:"encoding"`
	Label         string `json:"label"`
	NumberOfBytes string `json:"numberOfBytes"`
	Key           string `json:"key,omitempty"`
	Value         string `json:"value,omitempty"`
	Base          string `json:"base,omitempty"`
	Members       []StorageEntry `json:"members,omitempty"`
}

type EVMOutput struct {
	Bytecode         BytecodeOutput `json:"bytecode"`
	DeployedBytecode BytecodeOutput `json:"deployedBytecode"`
}

type BytecodeOutput struct {
	Object string `json:"object"`
}

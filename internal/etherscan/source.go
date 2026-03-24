package etherscan

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ilyapt/etherscan2sol/internal/solc"
)

// SourceFormat describes the format of source code returned by Etherscan.
type SourceFormat int

const (
	FormatFlattened    SourceFormat = iota // Plain Solidity text
	FormatMultiFile                       // Double-brace wrapped JSON
	FormatStandardJSON                    // Standard JSON input
)

// DetectSourceFormat determines how the source code is encoded.
func DetectSourceFormat(sourceCode string) SourceFormat {
	trimmed := strings.TrimSpace(sourceCode)
	if strings.HasPrefix(trimmed, "{{") {
		return FormatMultiFile
	}
	if strings.HasPrefix(trimmed, "{") {
		return FormatStandardJSON
	}
	return FormatFlattened
}

// NormalizeSource converts an Etherscan result into solc standard-json input.
// It returns the standard JSON input, the contract name, and any error.
func NormalizeSource(result *Result) (*solc.StandardJSONInput, string, error) {
	format := DetectSourceFormat(result.SourceCode)
	settings := buildSettings(result)

	switch format {
	case FormatFlattened:
		return normalizeFlattened(result, settings)
	case FormatMultiFile:
		return normalizeMultiFile(result, settings)
	case FormatStandardJSON:
		return normalizeStandardJSON(result, settings)
	default:
		return nil, "", fmt.Errorf("unknown source format")
	}
}

func normalizeFlattened(result *Result, settings solc.CompilerSettings) (*solc.StandardJSONInput, string, error) {
	fileName := result.ContractName + ".sol"
	input := &solc.StandardJSONInput{
		Language: "Solidity",
		Sources: map[string]solc.SourceEntry{
			fileName: {Content: result.SourceCode},
		},
		Settings: settings,
	}
	return input, result.ContractName, nil
}

func normalizeMultiFile(result *Result, settings solc.CompilerSettings) (*solc.StandardJSONInput, string, error) {
	// Strip the outer braces: "{{...}}" → "{...}"
	trimmed := strings.TrimSpace(result.SourceCode)
	inner := trimmed[1 : len(trimmed)-1]

	// Try parsing as full standard-json structure with "sources" key.
	var full struct {
		Sources  map[string]solc.SourceEntry `json:"sources"`
		Settings json.RawMessage             `json:"settings"`
	}
	if err := json.Unmarshal([]byte(inner), &full); err != nil {
		return nil, "", fmt.Errorf("parsing multi-file source: %w", err)
	}

	if full.Sources != nil {
		// Has structured format with "sources" key.
		// If embedded settings exist, parse and preserve them (remappings, viaIR, etc.),
		// only overriding outputSelection to ensure we get bytecode.
		finalSettings := settings
		if len(full.Settings) > 0 {
			if err := json.Unmarshal(full.Settings, &finalSettings); err == nil {
				finalSettings.OutputSelection = settings.OutputSelection
			}
		}
		input := &solc.StandardJSONInput{
			Language: "Solidity",
			Sources:  full.Sources,
			Settings: finalSettings,
		}
		return input, result.ContractName, nil
	}

	// Plain source map: {"file.sol": {"content": "..."}}
	var sourceMap map[string]solc.SourceEntry
	if err := json.Unmarshal([]byte(inner), &sourceMap); err != nil {
		return nil, "", fmt.Errorf("parsing multi-file source map: %w", err)
	}

	input := &solc.StandardJSONInput{
		Language: "Solidity",
		Sources:  sourceMap,
		Settings: settings,
	}
	return input, result.ContractName, nil
}

func normalizeStandardJSON(result *Result, settings solc.CompilerSettings) (*solc.StandardJSONInput, string, error) {
	var input solc.StandardJSONInput
	if err := json.Unmarshal([]byte(result.SourceCode), &input); err != nil {
		return nil, "", fmt.Errorf("parsing standard JSON source: %w", err)
	}

	// Preserve the original settings (remappings, viaIR, metadata, libraries, etc.)
	// but ensure our outputSelection is set so we get the bytecode we need.
	input.Settings.OutputSelection = settings.OutputSelection

	return &input, result.ContractName, nil
}

func buildSettings(result *Result) solc.CompilerSettings {
	enabled := result.OptimizationUsed == "1"

	runs := 200
	if parsed, err := strconv.Atoi(result.Runs); err == nil {
		runs = parsed
	}

	settings := solc.CompilerSettings{
		Optimizer: solc.OptimizerSettings{
			Enabled: enabled,
			Runs:    runs,
		},
		OutputSelection: map[string]map[string][]string{
			"*": {
				"*": {"abi", "evm.bytecode"},
			},
		},
	}

	if result.EVMVersion != "" && result.EVMVersion != "Default" {
		settings.EVMVersion = result.EVMVersion
	}

	return settings
}

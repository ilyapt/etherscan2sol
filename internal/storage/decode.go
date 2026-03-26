package storage

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"unicode"

	"github.com/ilyapt/etherscan2sol/internal/solc"
)

// Decode extracts and decodes a value from a raw 32-byte storage word.
func Decode(raw [32]byte, slot ResolvedSlot, types map[string]solc.StorageType) (*DecodedValue, error) {
	typeInfo, ok := types[slot.TypeKey]
	if !ok {
		return nil, fmt.Errorf("unknown type key: %s", slot.TypeKey)
	}

	// For dynamic string/bytes, we need the full raw word before extracting.
	if slot.TypeKey == "t_string_storage" || slot.TypeKey == "t_bytes_storage" {
		val := decodeDynamic(raw)
		return &DecodedValue{
			Label: slot.Label,
			Type:  typeInfo.Label,
			Value: val,
		}, nil
	}

	// Extract the relevant bytes from the 32-byte word.
	start := 32 - slot.Offset - slot.NumberOfBytes
	end := 32 - slot.Offset
	extracted := raw[start:end]

	val := decodeByType(extracted, slot, typeInfo)

	return &DecodedValue{
		Label: slot.Label,
		Type:  typeInfo.Label,
		Value: val,
	}, nil
}

func decodeByType(extracted []byte, slot ResolvedSlot, typeInfo solc.StorageType) any {
	key := slot.TypeKey

	switch {
	case strings.HasPrefix(key, "t_uint"):
		return decodeUint(extracted)

	case isSignedInt(key):
		return decodeInt(extracted, slot.NumberOfBytes)

	case key == "t_address":
		return decodeAddress(extracted)

	case key == "t_bool":
		return decodeBool(extracted)

	case isFixedBytes(key):
		return "0x" + hex.EncodeToString(extracted)

	case strings.HasPrefix(key, "t_enum"):
		return decodeUint(extracted)

	case strings.HasPrefix(key, "t_contract"):
		return decodeAddress(extracted)

	default:
		return "0x" + hex.EncodeToString(extracted)
	}
}

// isSignedInt returns true for t_int8, t_int256, etc. but not t_interface.
func isSignedInt(key string) bool {
	if !strings.HasPrefix(key, "t_int") {
		return false
	}
	rest := key[len("t_int"):]
	return len(rest) > 0 && unicode.IsDigit(rune(rest[0]))
}

// isFixedBytes returns true for t_bytes1..t_bytes32 but not t_bytes_storage.
func isFixedBytes(key string) bool {
	if !strings.HasPrefix(key, "t_bytes") {
		return false
	}
	rest := key[len("t_bytes"):]
	return len(rest) > 0 && unicode.IsDigit(rune(rest[0]))
}

func decodeUint(data []byte) string {
	v := new(big.Int).SetBytes(data)
	return v.Text(10)
}

func decodeInt(data []byte, numberOfBytes int) string {
	v := new(big.Int).SetBytes(data)
	bitSize := uint(numberOfBytes * 8)
	// If the top bit is set, the value is negative in two's complement.
	if v.Bit(int(bitSize)-1) != 0 {
		// subtract 2^bitSize
		modulus := new(big.Int).Lsh(big.NewInt(1), bitSize)
		v.Sub(v, modulus)
	}
	return v.Text(10)
}

func decodeAddress(data []byte) string {
	// Pad to 20 bytes if shorter.
	if len(data) < 20 {
		padded := make([]byte, 20)
		copy(padded[20-len(data):], data)
		data = padded
	}
	return "0x" + hex.EncodeToString(data)
}

func decodeBool(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return true
		}
	}
	return false
}

func decodeDynamic(raw [32]byte) string {
	// Check lowest bit of the last byte.
	if raw[31]&1 == 0 {
		// Short string: length = last_byte / 2, content in raw[0:length].
		length := int(raw[31]) / 2
		if length > 31 {
			length = 31
		}
		return string(raw[:length])
	}
	// Long string: actual length = (value - 1) / 2.
	v := new(big.Int).SetBytes(raw[:])
	v.Sub(v, big.NewInt(1))
	v.Div(v, big.NewInt(2))
	return fmt.Sprintf("(long string, length=%s)", v.Text(10))
}

package storage

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/sha3"
)

// Keccak256 computes the Keccak-256 hash.
func Keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

// EncodeKey encodes a CLI key argument as 32 bytes for storage slot computation.
// typeLabel is the solc type label like "t_address", "t_uint256", "t_bool", etc.
func EncodeKey(key string, typeLabel string) ([]byte, error) {
	switch {
	case strings.HasPrefix(typeLabel, "t_address"):
		return encodeAddress(key)
	case strings.HasPrefix(typeLabel, "t_uint"):
		return encodeUint(key)
	case strings.HasPrefix(typeLabel, "t_int") && !strings.HasPrefix(typeLabel, "t_interface"):
		return encodeInt(key)
	case strings.HasPrefix(typeLabel, "t_bool"):
		return encodeBool(key)
	case strings.HasPrefix(typeLabel, "t_bytes_storage") || strings.HasPrefix(typeLabel, "t_string"):
		return encodeStringOrDynamicBytes(key)
	case strings.HasPrefix(typeLabel, "t_bytes"):
		return encodeFixedBytes(key)
	case strings.HasPrefix(typeLabel, "t_enum"):
		return encodeUint(key)
	default:
		return nil, fmt.Errorf("unsupported type label: %s", typeLabel)
	}
}

// ComputeMappingSlot computes keccak256(abi.encode(key, slot)) for a mapping.
func ComputeMappingSlot(baseSlot *big.Int, encodedKey []byte) *big.Int {
	slotBytes := slotToBytes(baseSlot)
	data := make([]byte, 64)
	copy(data[:32], encodedKey)
	copy(data[32:], slotBytes[:])
	hash := Keccak256(data)
	return new(big.Int).SetBytes(hash)
}

// ComputeArrayElementSlot computes keccak256(slot) + index * elementSlots for a dynamic array.
func ComputeArrayElementSlot(baseSlot *big.Int, index uint64, elementSlots uint64) *big.Int {
	slotBytes := slotToBytes(baseSlot)
	hash := Keccak256(slotBytes[:])
	base := new(big.Int).SetBytes(hash)
	offset := new(big.Int).SetUint64(index)
	offset.Mul(offset, new(big.Int).SetUint64(elementSlots))
	return base.Add(base, offset)
}

func slotToBytes(slot *big.Int) [32]byte {
	var buf [32]byte
	b := slot.Bytes()
	copy(buf[32-len(b):], b)
	return buf
}

func encodeAddress(key string) ([]byte, error) {
	key = strings.TrimPrefix(key, "0x")
	key = strings.TrimPrefix(key, "0X")
	b, err := hex.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("invalid address hex: %w", err)
	}
	if len(b) > 32 {
		return nil, fmt.Errorf("address too long: %d bytes", len(b))
	}
	return leftPad(b, 32), nil
}

func encodeUint(key string) ([]byte, error) {
	n := new(big.Int)
	if strings.HasPrefix(key, "0x") || strings.HasPrefix(key, "0X") {
		_, ok := n.SetString(key[2:], 16)
		if !ok {
			return nil, fmt.Errorf("invalid hex uint: %s", key)
		}
	} else {
		_, ok := n.SetString(key, 10)
		if !ok {
			return nil, fmt.Errorf("invalid uint: %s", key)
		}
	}
	if n.Sign() < 0 {
		return nil, fmt.Errorf("uint cannot be negative: %s", key)
	}
	return leftPad(n.Bytes(), 32), nil
}

func encodeInt(key string) ([]byte, error) {
	n := new(big.Int)
	_, ok := n.SetString(key, 10)
	if !ok {
		return nil, fmt.Errorf("invalid int: %s", key)
	}
	// Two's complement 256-bit representation.
	if n.Sign() < 0 {
		// Add 2^256 to get the two's complement.
		mod := new(big.Int).Lsh(big.NewInt(1), 256)
		n.Add(n, mod)
	}
	return leftPad(n.Bytes(), 32), nil
}

func encodeBool(key string) ([]byte, error) {
	var v byte
	switch strings.ToLower(key) {
	case "true", "1":
		v = 1
	case "false", "0":
		v = 0
	default:
		return nil, fmt.Errorf("invalid bool: %s", key)
	}
	return leftPad([]byte{v}, 32), nil
}

func encodeFixedBytes(key string) ([]byte, error) {
	key = strings.TrimPrefix(key, "0x")
	key = strings.TrimPrefix(key, "0X")
	b, err := hex.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("invalid bytes hex: %w", err)
	}
	if len(b) > 32 {
		return nil, fmt.Errorf("fixed bytes too long: %d bytes", len(b))
	}
	return rightPad(b, 32), nil
}

func encodeStringOrDynamicBytes(key string) ([]byte, error) {
	hash := Keccak256([]byte(key))
	return hash, nil
}

func leftPad(b []byte, size int) []byte {
	if len(b) >= size {
		return b[len(b)-size:]
	}
	buf := make([]byte, size)
	copy(buf[size-len(b):], b)
	return buf
}

func rightPad(b []byte, size int) []byte {
	if len(b) >= size {
		return b[:size]
	}
	buf := make([]byte, size)
	copy(buf, b)
	return buf
}

package storage

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ilyapt/etherscan2sol/internal/solc"
)

// Resolve translates a variable name and optional keys into concrete storage slot(s).
// Returns one slot for simple values, multiple slots for structs (when no field specified).
func Resolve(layout solc.StorageLayout, variable string, keys []string) ([]ResolvedSlot, error) {
	// Step 1: find the storage entry by label.
	var entry *solc.StorageEntry
	for i := range layout.Storage {
		if layout.Storage[i].Label == variable {
			entry = &layout.Storage[i]
			break
		}
	}
	if entry == nil {
		names := make([]string, len(layout.Storage))
		for i, e := range layout.Storage {
			names[i] = e.Label
		}
		return nil, fmt.Errorf("variable %q not found; available: %s", variable, strings.Join(names, ", "))
	}

	// Step 2: parse slot.
	currentSlot, ok := new(big.Int).SetString(entry.Slot, 10)
	if !ok {
		return nil, fmt.Errorf("invalid slot %q for variable %q", entry.Slot, variable)
	}
	offset := entry.Offset
	currentType := entry.Type
	remaining := keys

	// Step 4: walk through keys.
	for {
		typeInfo, exists := layout.Types[currentType]
		if !exists {
			return nil, fmt.Errorf("type %q not found in layout types", currentType)
		}

		switch {
		case typeInfo.Encoding == "mapping":
			if len(remaining) == 0 {
				return nil, fmt.Errorf("mapping %q requires a key", variable)
			}
			keyTypeInfo, exists := layout.Types[typeInfo.Key]
			if !exists {
				return nil, fmt.Errorf("key type %q not found in layout types", typeInfo.Key)
			}
			encodedKey, err := EncodeKey(remaining[0], keyTypeInfo.Label)
			if err != nil {
				return nil, fmt.Errorf("encoding key %q as %s: %w", remaining[0], keyTypeInfo.Label, err)
			}
			currentSlot = ComputeMappingSlot(currentSlot, encodedKey)
			offset = 0
			currentType = typeInfo.Value
			remaining = remaining[1:]

		case typeInfo.Encoding == "dynamic_array":
			if len(remaining) == 0 {
				return nil, fmt.Errorf("dynamic array %q requires an index or 'length'", variable)
			}
			if remaining[0] == "length" {
				return []ResolvedSlot{{
					Slot:          currentSlot,
					Offset:        0,
					NumberOfBytes: 32,
					TypeKey:       "t_uint256",
					Label:         "length",
				}}, nil
			}
			index, err := strconv.ParseUint(remaining[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid array index %q: %w", remaining[0], err)
			}
			elementType, exists := layout.Types[typeInfo.Base]
			if !exists {
				return nil, fmt.Errorf("base type %q not found in layout types", typeInfo.Base)
			}
			elementSize, err := strconv.Atoi(elementType.NumberOfBytes)
			if err != nil {
				return nil, fmt.Errorf("invalid numberOfBytes %q for type %q: %w", elementType.NumberOfBytes, typeInfo.Base, err)
			}
			elementSlots := uint64((elementSize + 31) / 32)
			currentSlot = ComputeArrayElementSlot(currentSlot, index, elementSlots)
			offset = 0
			currentType = typeInfo.Base
			remaining = remaining[1:]

		case typeInfo.Encoding == "inplace" && len(typeInfo.Members) > 0:
			if len(remaining) > 0 {
				// Find the named member.
				var member *solc.StorageEntry
				for i := range typeInfo.Members {
					if typeInfo.Members[i].Label == remaining[0] {
						member = &typeInfo.Members[i]
						break
					}
				}
				if member == nil {
					fieldNames := make([]string, len(typeInfo.Members))
					for i, m := range typeInfo.Members {
						fieldNames[i] = m.Label
					}
					return nil, fmt.Errorf("field %q not found in struct; available: %s", remaining[0], strings.Join(fieldNames, ", "))
				}
				memberSlot, ok := new(big.Int).SetString(member.Slot, 10)
				if !ok {
					return nil, fmt.Errorf("invalid member slot %q", member.Slot)
				}
				currentSlot = new(big.Int).Add(currentSlot, memberSlot)
				offset = member.Offset
				currentType = member.Type
				remaining = remaining[1:]
			} else {
				// Return all members.
				slots := make([]ResolvedSlot, len(typeInfo.Members))
				for i, m := range typeInfo.Members {
					memberSlot, ok := new(big.Int).SetString(m.Slot, 10)
					if !ok {
						return nil, fmt.Errorf("invalid member slot %q", m.Slot)
					}
					nob, err := parseNumberOfBytes(layout, m.Type)
					if err != nil {
						return nil, err
					}
					slots[i] = ResolvedSlot{
						Slot:          new(big.Int).Add(currentSlot, memberSlot),
						Offset:        m.Offset,
						NumberOfBytes: nob,
						TypeKey:       m.Type,
						Label:         m.Label,
					}
				}
				return slots, nil
			}

		default:
			// Simple inplace type or bytes encoding — terminal.
			if len(remaining) > 0 {
				return nil, fmt.Errorf("type %q (%s) does not support further keys", currentType, typeInfo.Encoding)
			}
			nob, err := parseNumberOfBytes(layout, currentType)
			if err != nil {
				return nil, err
			}
			return []ResolvedSlot{{
				Slot:          currentSlot,
				Offset:        offset,
				NumberOfBytes: nob,
				TypeKey:       currentType,
				Label:         variable,
			}}, nil
		}
	}
}

func parseNumberOfBytes(layout solc.StorageLayout, typeKey string) (int, error) {
	t, exists := layout.Types[typeKey]
	if !exists {
		return 0, fmt.Errorf("type %q not found in layout types", typeKey)
	}
	n, err := strconv.Atoi(t.NumberOfBytes)
	if err != nil {
		return 0, fmt.Errorf("invalid numberOfBytes %q for type %q: %w", t.NumberOfBytes, typeKey, err)
	}
	return n, nil
}

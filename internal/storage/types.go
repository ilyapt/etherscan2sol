package storage

import "math/big"

// ResolvedSlot represents a fully resolved storage location.
type ResolvedSlot struct {
	Slot          *big.Int // the 256-bit slot number
	Offset        int      // byte offset within the 32-byte word (for packed vars)
	NumberOfBytes int      // how many bytes this value occupies
	TypeKey       string   // type key like "t_uint256" for lookup in Types map
	Label         string   // field name (for struct output)
}

// DecodedValue represents a decoded storage value.
type DecodedValue struct {
	Label string
	Type  string // human-readable type label
	Value any    // decoded Go value
}

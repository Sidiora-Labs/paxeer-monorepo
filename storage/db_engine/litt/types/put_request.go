package types

// A request to put a key-value pair with optional secondary keys into the database.
type PutRequest struct {
	// Key is the primary key, at most 65,535 bytes (math.MaxUint16).
	Key []byte
	// Value is the value to put, at most 4,294,967,295 bytes (math.MaxUint32).
	// Only written once, even if secondary keys are provided.
	Value []byte
	// Secondary keys pointing to sub-ranges of the value. Each key is at most 65,535 bytes (math.MaxUint16).
	// May be nil.
	SecondaryKeys []*SecondaryKey
}

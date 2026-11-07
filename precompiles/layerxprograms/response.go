package layerxprograms

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math/big"
)

const terminalFrameDomain = "LXP/programs/terminal-applied-legs/v1\x00"
const terminalFrameLimit = 1_048_576

// ProgramExecutionResult retains the complete terminal evidence as well as the
// nested guest response. ResponseCapacity applies only to GuestResponse.
type ProgramExecutionResult struct {
	TerminalResult      int32
	TerminalKind        uint8
	TerminalFrame       []byte
	Execution           []byte
	AppliedLegs         []byte
	GuestResponse       []byte
	SandboxUsageReceipt []byte
}

// DecodeProgramExecutionResult decodes call and simulateCall execution returns.
// transferSetRoot must come from independently verified receipt evidence (or a
// trusted local Host.Simulate result), never from the frame being decoded. This
// checks encoding and transfer consistency; receipt/authority authentication
// remains the caller's responsibility. Non-execution refusal details are refused.
func DecodeProgramExecutionResult(encoded []byte, responseCapacity uint32, transferSetRoot [32]byte) (ProgramExecutionResult, error) {
	var out ProgramExecutionResult
	invalid := errors.New("LXP_NON_CANONICAL: invalid programs execution result")
	if responseCapacity > terminalFrameLimit || len(encoded) < 128 || len(encoded) > 128+terminalFrameLimit || len(encoded)%32 != 0 {
		return out, invalid
	}
	outputs := programsABI.Methods["call"].Outputs
	values, err := outputs.Unpack(encoded)
	if err != nil {
		return out, invalid
	}
	status, kind, frame := values[0].(int32), values[1].(uint8), values[2].([]byte)
	canonical, err := outputs.Pack(status, kind, frame)
	if err != nil || !bytes.Equal(canonical, encoded) || kind < 1 || kind > 3 || (kind == 1) != (status == 0) || len(frame) > terminalFrameLimit {
		return out, invalid
	}
	c := responseCursor{raw: frame}
	if !bytes.Equal(c.take(uint64(len(terminalFrameDomain))), []byte(terminalFrameDomain)) {
		return out, invalid
	}
	execution, legs := c.sized32(terminalFrameLimit), c.sized32(256*115)
	if !c.end() || len(execution) == 0 || len(legs)%115 != 0 {
		return out, invalid
	}
	root, ok := appliedLegsRoot(legs)
	if !ok || root != transferSetRoot {
		return out, invalid
	}
	inner := execution
	const authority = "LXP/program-execution-with-transfer-authority/v2\x00"
	if bytes.HasPrefix(inner, []byte(authority)) {
		wrapper := responseCursor{raw: inner[len(authority):]}
		inner = wrapper.sized32(terminalFrameLimit)
		authorization := wrapper.sized32(terminalFrameLimit)
		boundRoot := wrapper.take(32)
		if !wrapper.end() || len(inner) == 0 || len(authorization) == 0 || !bytes.Equal(boundRoot, root[:]) {
			return out, invalid
		}
	}
	var sandbox *sandboxResponseReceipt
	const sandboxDomain = "LXP/programs/sandbox-settlement/v1\x00"
	if bytes.HasPrefix(inner, []byte(sandboxDomain)) {
		wrapper := responseCursor{raw: inner[len(sandboxDomain):]}
		inner = wrapper.sized32(terminalFrameLimit)
		receipt := wrapper.sized32(4096)
		boundRoot := wrapper.take(32)
		if !wrapper.end() || len(inner) == 0 || !bytes.Equal(boundRoot, root[:]) {
			return out, invalid
		}
		sandbox = decodeSandboxResponseReceipt(receipt, kind, root)
		if sandbox == nil || len(legs) != 115 || !bytes.Equal(sandbox.destination, legs[33:65]) || !bytes.Equal(sandbox.charged, legs[97:113]) {
			return out, invalid
		}
	}
	const occupancy = "LXP/program-execution-with-occupancy/v1\x00"
	if bytes.HasPrefix(inner, []byte(occupancy)) {
		wrapper := responseCursor{raw: inner[len(occupancy):]}
		inner = wrapper.sized32(terminalFrameLimit)
		attachment := wrapper.sized32(terminalFrameLimit)
		if !wrapper.end() || len(inner) == 0 || len(attachment) == 0 && (sandbox == nil || !sandbox.zeroOccupancy) {
			return out, invalid
		}
	}
	guest, ok := executionGuestResponse(inner, responseCapacity, kind, sandbox)
	if !ok {
		return out, invalid
	}
	out = ProgramExecutionResult{TerminalResult: status, TerminalKind: kind,
		TerminalFrame: bytes.Clone(frame), Execution: bytes.Clone(execution),
		AppliedLegs: bytes.Clone(legs), GuestResponse: bytes.Clone(guest)}
	if sandbox != nil {
		out.SandboxUsageReceipt = bytes.Clone(sandbox.framed)
	}
	return out, nil
}

type sandboxResponseReceipt struct {
	framed        []byte
	program       []byte
	usage         [6]uint64
	feeVersion    uint64
	fee           []byte
	destination   []byte
	charged       []byte
	zeroOccupancy bool
}

// The runtime authenticates the native receipt-plus-digest frame and publishes
// only its canonical body. Reconstruct the native LXUR frame for callers to bind
// to independently verified usage evidence; the digest alone is not authority.
func decodeSandboxResponseReceipt(canonical []byte, kind uint8, root [32]byte) *sandboxResponseReceipt {
	const domain = "LayerX/programs/sandbox/usage-receipt/v4\x00"
	const canonicalLength = len(domain) + 32 + 8 + 8 + 7*32 + 1 + 32 + 6*8 + 3*16 + 8*8 + 4 + 7*8 + 2*16 + 32
	if len(canonical) != canonicalLength || root == ([32]byte{}) {
		return nil
	}
	digest := sha256.Sum256(canonical)
	framed := append(bytes.Clone(canonical), digest[:]...)
	c := responseCursor{raw: canonical}
	if !bytes.Equal(c.take(uint64(len(domain))), []byte(domain)) || !nonzeroResponseID(c.take(32)) || c.number(8) == 0 || c.number(8) == 0 {
		return nil
	}
	r := &sandboxResponseReceipt{framed: framed}
	for i := 0; i < 7; i++ {
		field := c.take(32)
		if i < 5 && !nonzeroResponseID(field) {
			return nil
		}
		if i == 4 {
			r.destination = field
		}
	}
	if c.number(1) != uint64(kind) {
		return nil
	}
	r.program = c.take(32)
	if !nonzeroResponseID(r.program) {
		return nil
	}
	for i := range r.usage {
		r.usage[i] = c.number(8)
	}
	if r.usage[4] > 1<<32-1 {
		return nil
	}
	occupancy, occupancyFee := new(big.Int).SetBytes(c.take(16)), new(big.Int).SetBytes(c.take(16))
	r.zeroOccupancy = occupancy.Sign() == 0 && occupancyFee.Sign() == 0
	r.fee = c.take(16)
	for i := 0; i < 8; i++ {
		cumulative := c.number(8)
		if i < len(r.usage) && cumulative < r.usage[i] {
			return nil
		}
	}
	r.feeVersion = c.number(4)
	if r.feeVersion == 0 {
		return nil
	}
	executionFee := new(big.Int)
	for i := 0; i < 7; i++ {
		price := c.number(8)
		if price == 0 {
			return nil
		}
		if i < len(r.usage) {
			executionFee.Add(executionFee, new(big.Int).Mul(new(big.Int).SetUint64(r.usage[i]), new(big.Int).SetUint64(price)))
		} else if new(big.Int).Mul(occupancy, new(big.Int).SetUint64(price)).Cmp(occupancyFee) != 0 {
			return nil
		}
	}
	r.charged = c.take(16)
	charged, spent := new(big.Int).SetBytes(r.charged), new(big.Int).SetBytes(c.take(16))
	if executionFee.Cmp(new(big.Int).SetBytes(r.fee)) != 0 || charged.Sign() <= 0 ||
		charged.Cmp(new(big.Int).Add(executionFee, occupancyFee)) != 0 || spent.Cmp(charged) < 0 ||
		!bytes.Equal(c.take(32), root[:]) || !c.end() {
		return nil
	}
	return r
}

func nonzeroResponseID(id []byte) bool {
	return len(id) == 32 && !bytes.Equal(id, make([]byte, 32))
}

func appliedLegsRoot(legs []byte) ([32]byte, bool) {
	var zero [32]byte
	leaves := make([][32]byte, 0, len(legs)/115)
	for offset := 0; offset < len(legs); offset += 115 {
		leg := legs[offset : offset+115]
		if leg[0] != 0 || leg[113] != 0 || leg[114] != 1 ||
			bytes.Equal(leg[1:33], make([]byte, 32)) || bytes.Equal(leg[33:65], make([]byte, 32)) ||
			bytes.Equal(leg[65:97], make([]byte, 32)) || bytes.Equal(leg[97:113], make([]byte, 16)) {
			return zero, false
		}
		leaves = append(leaves, sha256.Sum256(append([]byte("LXP/v1/merkle-leaf\x00"), leg...)))
	}
	for len(leaves) > 1 {
		next := make([][32]byte, 0, (len(leaves)+1)/2)
		for index := 0; index < len(leaves); index += 2 {
			right := leaves[index]
			if index+1 < len(leaves) {
				right = leaves[index+1]
			}
			input := append([]byte("LXP/v1/merkle-internal\x00"), leaves[index][:]...)
			next = append(next, sha256.Sum256(append(input, right[:]...)))
		}
		leaves = next
	}
	if len(leaves) == 0 {
		return zero, true
	}
	return leaves[0], true
}

// responseCursor refuses truncation before converting untrusted lengths to int.
type responseCursor struct {
	raw    []byte
	failed bool
}

func (c *responseCursor) take(length uint64) []byte {
	if c.failed || length > uint64(len(c.raw)) {
		c.failed = true
		return nil
	}
	value := c.raw[:int(length)]
	c.raw = c.raw[int(length):]
	return value
}
func (c *responseCursor) number(length uint64) uint64 {
	var value uint64
	for _, word := range c.take(length) {
		value = value<<8 | uint64(word)
	}
	return value
}
func (c *responseCursor) sized32(limit uint64) []byte { return c.sized(4, limit) }
func (c *responseCursor) sized64(limit uint64) []byte { return c.sized(8, limit) }
func (c *responseCursor) sized(width, limit uint64) []byte {
	length := c.number(width)
	if length > limit {
		c.failed = true
		return nil
	}
	return c.take(length)
}
func (c *responseCursor) end() bool { return !c.failed && len(c.raw) == 0 }

func executionGuestResponse(encoded []byte, capacity uint32, kind uint8, sandbox *sandboxResponseReceipt) ([]byte, bool) {
	version := 0
	c := responseCursor{}
	for _, candidate := range []struct {
		version int
		domain  string
	}{
		{2, "LXP/program-execution/v2\x00"}, {3, "LXP/program-execution/v3\x00"},
		{4, "LXP/program-execution/v4\x00"}, {5, "LXP/program-execution/v5\x00"},
	} {
		if bytes.HasPrefix(encoded, []byte(candidate.domain)) {
			version = candidate.version
			c.raw = encoded[len(candidate.domain):]
			break
		}
	}
	if version == 0 || c.number(2) == 0 {
		return nil, false
	}
	if version <= 3 {
		if kind != 1 || c.number(2) != 1 || c.number(4) == 0 {
			return nil, false
		}
	} else {
		feeVersion := c.number(4)
		if feeVersion == 0 || c.number(4) == 0 || sandbox != nil && feeVersion != sandbox.feeVersion {
			return nil, false
		}
	}
	var count uint64
	if version <= 3 && c.number(8) != 0 {
		return nil, false
	}
	count = c.number(8)
	if count > uint64(len(c.raw)/5) {
		return nil, false
	}
	for i := uint64(0); i < count; i++ {
		switch c.number(1) {
		case 1:
			c.take(4)
		case 2:
			c.take(8)
		default:
			return nil, false
		}
	}
	var usage [7]uint64
	for i := 0; i < 4; i++ {
		usage[i] = c.number(8)
	}
	usage[4] = c.number(4)
	if version >= 4 {
		usage[5] = c.number(8)
	}
	fee := c.take(16)
	if sandbox != nil {
		if version < 4 || !bytes.Equal(fee, sandbox.fee) {
			return nil, false
		}
		for i, value := range sandbox.usage {
			if usage[i] != value {
				return nil, false
			}
		}
	}
	if version >= 3 {
		switch c.number(1) {
		case 0:
			if version == 3 {
				return nil, false
			}
		case 1:
			if len(c.sized64(34+65536*52)) == 0 {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	if version <= 3 {
		return nil, c.end()
	}
	program := c.take(32)
	if sandbox != nil && !bytes.Equal(program, sandbox.program) {
		return nil, false
	}
	abi := c.number(2)
	if len(program) != 32 || bytes.Equal(program, make([]byte, 32)) || (version == 4 && abi != 2) || (version == 5 && (abi < 3 || abi > 5)) {
		return nil, false
	}
	var guest []byte
	switch c.number(1) {
	case 0:
		if kind != 1 || c.number(4) > 0x7fffffff {
			return nil, false
		}
		guest = c.sized64(uint64(capacity))
	case 1:
		if kind != 2 {
			return nil, false
		}
		failure := responseCursor{raw: c.sized64(4136)}
		id, tag := failure.take(32), failure.number(4)
		reason := failure.sized32(4096)
		if !failure.end() || len(id) != 32 || bytes.Equal(id, make([]byte, 32)) ||
			!(tag >= 1 && tag <= 5 || tag == 254 || tag == 255) || (tag >= 254 && len(reason) != 0) {
			return nil, false
		}
	case 2:
		if kind != 3 {
			return nil, false
		}
		tag, resource := c.number(1), c.number(1)
		if resource > 6 {
			return nil, false
		}
		if tag == 0 {
			limit, attempted := c.number(8), c.number(8)
			if attempted <= limit || usage[resource] > limit {
				return nil, false
			}
		} else if tag != 1 {
			return nil, false
		}
	default:
		return nil, false
	}
	graph := c.sized64(terminalFrameLimit)
	if !bytes.HasPrefix(graph, []byte("LayerX/programs/call-graph/v1\x00")) {
		return nil, false
	}
	return guest, c.end()
}

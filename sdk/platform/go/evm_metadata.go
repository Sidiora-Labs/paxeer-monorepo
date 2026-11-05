package layerx

type EVMOperationMetadata struct {
	Method   string
	To       string
	Selector string
	PerLeg   bool
}

func (operation AgentOperation) EVMMetadata() (EVMOperationMetadata, bool) {
	switch operation {
	case AgentOperationProgramActivity:
		return EVMOperationMetadata{Method: "eth_getTransactionByHash", To: "", Selector: ""}, true
	case AgentOperationProgramCall:
		return EVMOperationMetadata{Method: "eth_sendRawTransaction", To: "0x000000000000000000000000000000000000101d", Selector: "call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)"}, true
	case AgentOperationProgramDeploy:
		return EVMOperationMetadata{Method: "eth_sendRawTransaction", To: "0x000000000000000000000000000000000000101d", Selector: "deploy(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes)"}, true
	case AgentOperationProgramDiscover:
		return EVMOperationMetadata{Method: "eth_call", To: "0x000000000000000000000000000000000000101d", Selector: "getProgram(bytes32)"}, true
	case AgentOperationProgramInterface:
		return EVMOperationMetadata{Method: "eth_call", To: "0x000000000000000000000000000000000000101d", Selector: "getInterface(bytes32)"}, true
	case AgentOperationProgramReceipt:
		return EVMOperationMetadata{Method: "eth_getTransactionByHash", To: "", Selector: ""}, true
	case AgentOperationProgramSimulate:
		return EVMOperationMetadata{Method: "eth_call", To: "0x000000000000000000000000000000000000101d", Selector: "simulateCall(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)"}, true
	case AgentOperationProgramUpgrade:
		return EVMOperationMetadata{Method: "eth_sendRawTransaction", To: "0x000000000000000000000000000000000000101d", Selector: "upgrade(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes,bytes,uint128)"}, true
	case AgentOperationProgramWindDown:
		return EVMOperationMetadata{Method: "eth_sendRawTransaction", To: "0x000000000000000000000000000000000000101d", Selector: "deprecate(bytes32,bytes32,uint64)"}, true
	default:
		return EVMOperationMetadata{}, false
	}
}

.PHONY: test protoc install-proto-dep

export GO111MODULE := on

test:
	go test .

# Independent protobuf generation is retired; see README.md.
protoc install-proto-dep:
	@echo "Local ICS23 protobuf generation is retired. Use github.com/cosmos/cosmos-sdk/ics23/go v0.8.0 selected by the root go.mod; see README.md." >&2
	@exit 1

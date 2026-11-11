package bindings

//go:generate python3 generate_native.py
//go:generate go run github.com/ethereum/go-ethereum/cmd/abigen --abi abi/native_launchpad.json --pkg bindings --type NativeLaunchpad --out native_launchpad.go

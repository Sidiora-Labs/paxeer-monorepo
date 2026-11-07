// Package interop contains the hosted interop deployment contracts.
package interop

import _ "embed"

// KubernetesDeployment is the configuration supplied to the interop container.
//
//go:embed deployment.yaml
var KubernetesDeployment []byte

// FlyDeployment is the configuration supplied to each interop Fly machine.
//
//go:embed fly.toml
var FlyDeployment []byte

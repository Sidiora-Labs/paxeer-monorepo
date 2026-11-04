package interop

import (
	"bytes"
	"io"
	"path"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRepairM_1350_KubernetesRetiredAuthority(t *testing.T) {
	// Decode the production embedded manifest, including every YAML document.
	decoder := yaml.NewDecoder(bytes.NewReader(KubernetesDeployment))
	var deployments []repair137Deployment
	for {
		var document repair137Deployment
		err := decoder.Decode(&document)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if document.Kind == "Deployment" {
			deployments = append(deployments, document)
		}
	}
	require.Len(t, deployments, 1)
	pod := deployments[0].Spec.Template.Spec
	require.Len(t, pod.Containers, 1)
	container := pod.Containers[0]
	env := map[string]string{}
	for _, entry := range container.Env {
		require.NotContains(t, env, entry.Name, "duplicate environment input")
		env[entry.Name] = entry.Value
	}
	repairM1350CurrentInputs(t, env, "https://layerx-node.layerx-testnet.svc.cluster.local:8545")

	secrets := map[string]string{}
	for _, volume := range pod.Volumes {
		require.NotContains(t, secrets, volume.Name, "duplicate volume")
		require.False(t, volume.Secret.Optional)
		secrets[volume.Name] = volume.Secret.Name
	}
	require.Equal(t, map[string]string{
		"server":       "layerx-interop-gateway-tls",
		"trust":        "layerx-internal-ca",
		"client":       "layerx-interop-client-identity",
		"receipt-keys": "layerx-interop-receipt-keys",
		"redis":        "layerx-interop-redis-client",
		"interop":      "layerx-interop-runtime",
	}, secrets, "only current runtime secrets may be required at pod startup")
	mounts := map[string]string{}
	for _, mount := range container.Mounts {
		require.True(t, mount.ReadOnly)
		require.Contains(t, secrets, mount.Name)
		require.NotContains(t, mounts, mount.Path, "duplicate mount destination")
		mounts[mount.Path] = mount.Name
	}
	require.Len(t, mounts, len(secrets), "no unused secret volume")
	for input, source := range map[string]string{
		"LAYERX_INTEROP_TLS_CERT_DER":                  "server",
		"LAYERX_INTEROP_TLS_KEY_DER":                   "server",
		"LAYERX_INTEROP_OUTBOUND_CA_DER":               "trust",
		"LAYERX_INTEROP_CLIENT_IDENTITY_PKCS12":        "client",
		"LAYERX_INTEROP_CLIENT_IDENTITY_PASSWORD_FILE": "client",
		"LAYERX_INTEROP_RECEIPT_KEYS_FILE":             "receipt-keys",
		"LAYERX_INTEROP_REDIS_USERNAME_FILE":           "redis",
		"LAYERX_INTEROP_REDIS_PASSWORD_FILE":           "redis",
		"LAYERX_INTEROP_CONFIG":                        "interop",
		"LAYERX_INTEROP_MODULE_REGISTRY_FILE":          "interop",
	} {
		require.Equal(t, source, mounts[path.Dir(env[input])], "%s must resolve to its current secret", input)
	}
	for _, volume := range pod.Volumes {
		if volume.Name == "receipt-keys" {
			require.EqualValues(t, 0440, volume.Secret.Mode)
			require.Len(t, volume.Secret.Items, 1)
			require.Equal(t, "receipt-keys.json", volume.Secret.Items[0].Key)
			require.Equal(t, path.Base(env["LAYERX_INTEROP_RECEIPT_KEYS_FILE"]), volume.Secret.Items[0].Path)
		}
	}
	require.NotContains(t, string(KubernetesDeployment), "/run/layerx/authority")
	require.NotContains(t, string(KubernetesDeployment), "layerx-gateway-authority-client")
	require.NotContains(t, string(KubernetesDeployment), "layerx-receipt-authority")
}

func TestRepairM_1350_FlyRetiredAuthority(t *testing.T) {
	var deployment struct {
		Env   map[string]string `toml:"env"`
		Files []struct {
			Path   string `toml:"guest_path"`
			Secret string `toml:"secret_name"`
		} `toml:"files"`
	}
	_, err := toml.Decode(string(FlyDeployment), &deployment)
	require.NoError(t, err)
	repairM1350CurrentInputs(t, deployment.Env, "https://paxeer-x-core.internal:8545")
	files := map[string]string{}
	for _, file := range deployment.Files {
		require.NotEmpty(t, file.Secret)
		require.NotContains(t, files, file.Path, "duplicate secret destination")
		files[file.Path] = file.Secret
	}
	require.Equal(t, map[string]string{
		"/run/layerx/interop/config.json":            "INTEROP_CONFIG",
		"/run/layerx/interop/registry.json":          "INTEROP_MODULE_REGISTRY",
		"/run/layerx/trust/ca.crt.der":               "INTEROP_CLIENT_CA_DER",
		"/run/layerx/client/client.p12":              "INTEROP_CLIENT_P12",
		"/run/layerx/client/password":                "INTEROP_CLIENT_PASSWORD",
		"/run/layerx/receipt-keys/receipt-keys.json": "INTEROP_RECEIPT_KEYS",
		"/run/layerx/redis/username":                 "INTEROP_REDIS_USERNAME",
		"/run/layerx/redis/password":                 "INTEROP_REDIS_PASSWORD",
	}, files, "machine initialization must require only the current runtime's app secrets")
	for input, secret := range map[string]string{
		"LAYERX_INTEROP_CONFIG":                        "INTEROP_CONFIG",
		"LAYERX_INTEROP_MODULE_REGISTRY_FILE":          "INTEROP_MODULE_REGISTRY",
		"LAYERX_INTEROP_OUTBOUND_CA_DER":               "INTEROP_CLIENT_CA_DER",
		"LAYERX_INTEROP_CLIENT_IDENTITY_PKCS12":        "INTEROP_CLIENT_P12",
		"LAYERX_INTEROP_CLIENT_IDENTITY_PASSWORD_FILE": "INTEROP_CLIENT_PASSWORD",
		"LAYERX_INTEROP_RECEIPT_KEYS_FILE":             "INTEROP_RECEIPT_KEYS",
		"LAYERX_INTEROP_REDIS_USERNAME_FILE":           "INTEROP_REDIS_USERNAME",
		"LAYERX_INTEROP_REDIS_PASSWORD_FILE":           "INTEROP_REDIS_PASSWORD",
	} {
		require.Equal(t, secret, files[deployment.Env[input]], "%s must resolve to its current app secret", input)
	}
	for _, retired := range []string{
		"/run/layerx/authority", "INTEROP_AUTHORITY_TOKEN", "INTEROP_SEQUENCER_PUBLIC_KEY",
		"INTEROP_SEQUENCER_ID", "INTEROP_SEQUENCER_FIRST_BATCH", "INTEROP_SEQUENCER_LAST_BATCH",
		"receipt authority", "router", "kernel",
	} {
		require.NotContains(t, string(FlyDeployment), retired)
	}
	require.Contains(t, string(FlyDeployment), "firstHeight and lastHeight")
	require.Contains(t, string(FlyDeployment), "Never derive these pins from the queried endpoint")
}

func repairM1350CurrentInputs(t *testing.T, env map[string]string, endpoint string) {
	t.Helper()
	for name := range env {
		require.False(t, strings.HasPrefix(name, "LAYERX_INTEROP_RECEIPT_AUTHORITY_"), name)
		if strings.HasPrefix(name, "LAYERX_INTEROP_SEQUENCER_") {
			require.Equal(t, "LAYERX_INTEROP_SEQUENCER_URL", name)
		}
	}
	require.Equal(t, endpoint, env["LAYERX_INTEROP_SEQUENCER_URL"])
	require.Equal(t, "/run/layerx/receipt-keys/receipt-keys.json", env["LAYERX_INTEROP_RECEIPT_KEYS_FILE"])
	require.Equal(t, "125", env["LAYERX_INTEROP_CHAIN_ID"])
	require.Equal(t, "15000", env["LAYERX_INTEROP_READINESS_MAX_AGE_MS"])
}

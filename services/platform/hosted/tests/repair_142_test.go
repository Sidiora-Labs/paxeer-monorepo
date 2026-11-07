package hostedtests

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	sequencer "github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"gopkg.in/yaml.v3"
)

func repair142Command(t *testing.T, args ...string) []byte {
	t.Helper()
	command := exec.Command(args[0], args[1:]...)
	command.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s\n%s", strings.Join(args, " "), output)
	return output
}

func repair142Write(t *testing.T, path string, data []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

func repair142Pin(seed byte, first, last uint64) seqkeeper.ReceiptKeyEntry {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, 32))
	id := sha256.Sum256(append([]byte("receipt-key:"), private.Public().(ed25519.PublicKey)...))
	return seqkeeper.ReceiptKeyEntry{KeyId: id[:], PublicKey: private.Public().(ed25519.PublicKey), FirstHeight: first, LastHeight: last}
}

func repair142Export(t *testing.T, path string, pins seqkeeper.ReceiptKeys) {
	t.Helper()
	database := dbm.NewMemDB()
	defer database.Close()
	multi := store.NewCommitMultiStore(database)
	key := sdk.NewKVStoreKey(seqtypes.StoreKey)
	multi.MountStoreWithDB(key, sdk.StoreTypeIAVL, database)
	require.NoError(t, multi.LoadLatestVersion())
	ctx := sdk.NewContext(multi, tmproto.Header{ChainID: "receipt-pins-test", Height: 1}, false)
	keeper := seqkeeper.NewKeeper(key)
	require.NoError(t, keeper.SetReceiptKeys(ctx, pins))
	multi.Commit(true)
	stored, err := keeper.ReceiptKeys(ctx)
	require.NoError(t, err)
	exported := make(seqkeeper.ReceiptKeys, len(stored))
	for i, pin := range stored {
		exported[i] = seqkeeper.ReceiptKeyEntry{KeyId: pin.KeyId[:], PublicKey: pin.PublicKey[:], FirstHeight: pin.FirstHeight, LastHeight: pin.LastHeight}
	}
	encoded, err := json.Marshal(exported)
	require.NoError(t, err)
	repair142Write(t, path, encoded)
}

type repair142Pod struct {
	Containers []struct {
		Name string `yaml:"name"`
		Env  []struct {
			Name  string `yaml:"name"`
			Value string `yaml:"value"`
		} `yaml:"env"`
		VolumeMounts []struct {
			Name      string `yaml:"name"`
			MountPath string `yaml:"mountPath"`
			ReadOnly  bool   `yaml:"readOnly"`
			SubPath   string `yaml:"subPath"`
		} `yaml:"volumeMounts"`
	} `yaml:"containers"`
	Volumes []struct {
		Name   string `yaml:"name"`
		Secret struct {
			SecretName  string `yaml:"secretName"`
			DefaultMode int    `yaml:"defaultMode"`
			Items       []struct {
				Key  string `yaml:"key"`
				Path string `yaml:"path"`
			} `yaml:"items"`
		} `yaml:"secret"`
	} `yaml:"volumes"`
}

func repair142GatewayPod(t *testing.T, path string) repair142Pod {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc struct {
			Kind     string `yaml:"kind"`
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
			Spec struct {
				Template struct {
					Spec repair142Pod `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"spec"`
		}
		err := decoder.Decode(&doc)
		if err == io.EOF {
			t.Fatal("gateway deployment missing")
		}
		require.NoError(t, err)
		if doc.Kind == "Deployment" && doc.Metadata.Name == "layerx-gateway" {
			return doc.Spec.Template.Spec
		}
	}
}

func repair142Project(t *testing.T, material, manifests string) string {
	t.Helper()
	var secret struct {
		Kind     string                           `json:"kind"`
		Metadata struct{ Name, Namespace string } `json:"metadata"`
		Data     map[string]string                `json:"data"`
	}
	require.NoError(t, json.Unmarshal(repair142Command(t, "python3", "sequencer-pins.py", "--gateway-secret", material, "layerx-testnet"), &secret))
	require.Equal(t, "Secret", secret.Kind)
	require.Equal(t, "layerx-testnet", secret.Metadata.Namespace)
	require.Len(t, secret.Data, 1)
	pod := repair142GatewayPod(t, filepath.Join(manifests, "gateway.yaml"))
	var configured, mountName, mountPath string
	for _, container := range pod.Containers {
		if container.Name != "gateway" {
			continue
		}
		for _, env := range container.Env {
			require.False(t, strings.HasPrefix(env.Name, "LAYERX_GATEWAY_SEQUENCER_"), env.Name)
			if env.Name == "LAYERX_GATEWAY_RECEIPT_KEYS_FILE" {
				require.Empty(t, configured)
				configured = env.Value
			}
		}
		for _, mount := range container.VolumeMounts {
			if mount.Name == "receipt-keys" {
				require.True(t, mount.ReadOnly)
				require.Empty(t, mount.SubPath, "rotation must project the whole secret volume")
				mountName, mountPath = mount.Name, mount.MountPath
			}
		}
	}
	require.Equal(t, "/run/layerx/receipt-keys/receipt-keys.json", configured)
	require.NotEmpty(t, mountName)
	for _, volume := range pod.Volumes {
		if volume.Name != mountName {
			continue
		}
		require.Equal(t, secret.Metadata.Name, volume.Secret.SecretName)
		require.Equal(t, 0o440, volume.Secret.DefaultMode)
		require.Len(t, volume.Secret.Items, 1)
		item := volume.Secret.Items[0]
		require.Equal(t, configured, mountPath+"/"+item.Path)
		encoded, exists := secret.Data[item.Key]
		require.True(t, exists, "projected key absent from produced secret")
		data, err := base64.StdEncoding.DecodeString(encoded)
		require.NoError(t, err)
		path := filepath.Join(material, "projected", strings.TrimPrefix(configured, "/"))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		repair142Write(t, path, data)
		return path
	}
	t.Fatal("receipt key volume missing")
	return ""
}

func repair142BuildConsumer(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	gateway, err := filepath.Abs("../gateway")
	require.NoError(t, err)
	source, err := filepath.Abs("receipt-pins-consumer.rs")
	require.NoError(t, err)
	manifest := fmt.Sprintf("[package]\nname = \"receipt-pins-consumer\"\nversion = \"0.1.0\"\nedition = \"2021\"\n[workspace]\n[[bin]]\nname = \"receipt-pins-consumer\"\npath = %q\n[dependencies]\nlayerx-platform-gateway = { path = %q }\n", source, gateway)
	path := filepath.Join(directory, "Cargo.toml")
	repair142Write(t, path, []byte(manifest))
	command := exec.Command("cargo", "build", "--quiet", "--manifest-path", path, "--target-dir", filepath.Join(directory, "target"))
	output, err := command.CombinedOutput()
	require.NoError(t, err, "compile real gateway receipt consumer:\n%s", output)
	return filepath.Join(directory, "target", "debug", "receipt-pins-consumer")
}

type repair142Consumer struct {
	command *exec.Cmd
	input   io.WriteCloser
	output  *bufio.Scanner
	stderr  bytes.Buffer
	stopped bool
}

func repair142Start(t *testing.T, binary, keys string, count int) *repair142Consumer {
	t.Helper()
	consumer := &repair142Consumer{command: exec.Command(binary)}
	consumer.command.Env = append(os.Environ(), "LAYERX_GATEWAY_RECEIPT_KEYS_FILE="+keys)
	consumer.command.Stderr = &consumer.stderr
	input, err := consumer.command.StdinPipe()
	require.NoError(t, err)
	output, err := consumer.command.StdoutPipe()
	require.NoError(t, err)
	consumer.input, consumer.output = input, bufio.NewScanner(output)
	require.NoError(t, consumer.command.Start())
	t.Cleanup(func() { consumer.stop(t) })
	require.True(t, consumer.output.Scan(), "consumer startup failed")
	require.Equal(t, fmt.Sprintf("ready:%d", count), consumer.output.Text())
	return consumer
}

func (consumer *repair142Consumer) stop(t *testing.T) {
	t.Helper()
	if consumer.stopped {
		return
	}
	consumer.stopped = true
	require.NoError(t, consumer.input.Close())
	require.NoError(t, consumer.command.Wait(), "%s", consumer.stderr.String())
}

func (consumer *repair142Consumer) query(t *testing.T, command string) string {
	t.Helper()
	_, err := fmt.Fprintln(consumer.input, command)
	require.NoError(t, err)
	require.True(t, consumer.output.Scan(), "consumer stopped unexpectedly")
	return consumer.output.Text()
}

func repair142Receipt(t *testing.T, directory string, seed byte, pin seqkeeper.ReceiptKeyEntry, height int64) string {
	t.Helper()
	// The node's real signer emits the wire receipt; its broad producer pin
	// also lets the gateway prove refusals outside the deployed pin window.
	keyFile := filepath.Join(directory, fmt.Sprintf("key-%d", seed))
	repair142Write(t, keyFile, []byte(hex.EncodeToString(bytes.Repeat([]byte{seed}, 32))))
	consensusFile := filepath.Join(directory, fmt.Sprintf("consensus-%d.json", seed))
	pv, err := privval.GenFilePV(consensusFile, filepath.Join(directory, "consensus-state"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Key.Save())
	key, err := sequencer.LoadReceiptKey(keyFile, consensusFile, []verify.SequencerAuthorization{{
		SequencerID: [32]byte(pin.KeyId), PublicKey: [32]byte(pin.PublicKey), FirstBatchNumber: 1, LastBatchNumber: math.MaxUint64,
	}}, height)
	require.NoError(t, err)
	activity := sha256.Sum256([]byte(fmt.Sprintf("receipt-%d-%d", seed, height)))
	preimage := append([]byte("PAXEER/v1/sequence\x00"), make([]byte, 32)...)
	preimage = binary.BigEndian.AppendUint64(preimage, 1)
	next := sha256.Sum256(append(preimage, activity[:]...))
	input := &sequencer.ReceiptInput{
		Accepted: &sequencer.Accepted{Seq: 1, SeqHash: next, ActivityID: activity, Height: height, TimestampMs: 1000},
		EVM:      &evmtypes.Receipt{Status: 1}, Operation: 2, ModuleID: 12,
		PaymentDenom: "upax", FeeDenom: "upax", FeeCharged: big.NewInt(0), Amount: big.NewInt(0),
		FromBalanceBefore: big.NewInt(0), FromBalanceAfter: big.NewInt(0), ToBalanceBefore: big.NewInt(0), ToBalanceAfter: big.NewInt(0),
	}
	receipt, err := key.SignReceipt(input)
	require.NoError(t, err)
	require.NotEmpty(t, receipt)
	path := filepath.Join(directory, fmt.Sprintf("receipt-%d-%d", seed, height))
	repair142Write(t, path, receipt)
	return path
}

func TestRepair_142_GatewayReceiptKeyProvisioning(t *testing.T) {
	directory := t.TempDir()
	source, output := filepath.Join(directory, "governed.json"), filepath.Join(directory, "receipt-keys.json")
	primary := repair142Pin(3, 1, math.MaxUint64)
	repair142Export(t, source, seqkeeper.ReceiptKeys{primary})
	repair142Command(t, "python3", "sequencer-pins.py", "--gateway", source, output)
	info, err := os.Stat(output)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	manifests := filepath.Join(directory, "manifests")
	require.NoError(t, os.Mkdir(manifests, 0o700))
	for destination, source := range map[string]string{"gateway.yaml": "../gateway/deployment.yaml", "developer.yaml": "../webhooks/deployment.yaml"} {
		data, err := os.ReadFile(source)
		require.NoError(t, err)
		repair142Write(t, filepath.Join(manifests, destination), data)
	}
	repair142Command(t, "python3", "sequencer-pins.py", "--manifests", manifests)
	before, err := os.ReadFile(filepath.Join(manifests, "gateway.yaml"))
	require.NoError(t, err)
	repair142Command(t, "python3", "sequencer-pins.py", "--manifests", manifests)
	after, err := os.ReadFile(filepath.Join(manifests, "gateway.yaml"))
	require.NoError(t, err)
	require.Equal(t, before, after, "manifest projection must be idempotent")
	projected := repair142Project(t, directory, manifests)
	binary := repair142BuildConsumer(t)
	consumer := repair142Start(t, binary, projected, 1)
	receipt := repair142Receipt(t, directory, 3, primary, 1)
	require.Equal(t, "verified:1", consumer.query(t, "verify "+receipt))
	consumer.stop(t)

	// Governance closes the old range above the current height and introduces
	// a new key. No identity or height is derived from the webhook key files.
	rotatedPrimary := primary
	rotatedPrimary.LastHeight = 6999
	standby := repair142Pin(4, 7000, math.MaxUint64)
	rotated := seqkeeper.ReceiptKeys{rotatedPrimary, standby}
	require.NoError(t, seqkeeper.ValidateRotation(seqkeeper.ReceiptKeys{primary}, rotated, 6000))
	repair142Export(t, source, rotated)
	repair142Command(t, "python3", "sequencer-pins.py", "--gateway", source, output)
	require.Equal(t, projected, repair142Project(t, directory, manifests))
	consumer = repair142Start(t, binary, projected, 2)
	for _, test := range []struct {
		seed     byte
		pin      seqkeeper.ReceiptKeyEntry
		height   int64
		accepted bool
	}{
		{3, primary, 1, true}, {3, primary, 6999, true}, {4, standby, 7000, true},
		{4, standby, math.MaxInt64, true}, {3, primary, 7000, false}, {4, standby, 6999, false},
		{5, repair142Pin(5, 1, math.MaxUint64), 7000, false},
	} {
		path := repair142Receipt(t, directory, test.seed, test.pin, test.height)
		answer := consumer.query(t, "verify "+path)
		if test.accepted {
			require.Equal(t, "verified:1", answer)
		} else {
			require.Equal(t, "refused:UntrustedSequencer", answer)
		}
	}
	require.Equal(t, "authorized", consumer.query(t, fmt.Sprintf("header %x %d", standby.KeyId, uint64(math.MaxUint64))))
	require.Equal(t, "refused:UntrustedSequencer", consumer.query(t, fmt.Sprintf("header %x 0", primary.KeyId)))
	corrupted, err := os.ReadFile(receipt)
	require.NoError(t, err)
	corrupted[len(corrupted)-1] ^= 1
	corruptPath := filepath.Join(directory, "corrupted-receipt")
	repair142Write(t, corruptPath, corrupted)
	require.Contains(t, consumer.query(t, "verify "+corruptPath), "refused:Receipt(SequencerSignature)")
	substituted := primary
	substituted.PublicKey = standby.PublicKey
	wrongSigner := repair142Receipt(t, directory, 4, substituted, 1)
	require.Equal(t, "refused:Receipt(SequencerSignature)", consumer.query(t, "verify "+wrongSigner))
	consumer.stop(t)

	saved, err := os.ReadFile(output)
	require.NoError(t, err)
	require.NoError(t, os.Remove(source))
	repair142Command(t, "python3", "sequencer-pins.py", "--gateway-check", output)
	require.Equal(t, projected, repair142Project(t, directory, manifests))
	reopened := repair142Start(t, binary, projected, 2)
	require.Equal(t, "verified:1", reopened.query(t, "verify "+receipt))
	retainedRotated := repair142Receipt(t, directory, 4, standby, 7000)
	require.Equal(t, "verified:1", reopened.query(t, "verify "+retainedRotated))
	reopened.stop(t)
	retained, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Equal(t, saved, retained, "retained startup must not regenerate pins")
}

func TestRepair_142_RefusesMalformedReceiptKeys(t *testing.T) {
	directory := t.TempDir()
	source, output := filepath.Join(directory, "source.json"), filepath.Join(directory, "receipt-keys.json")
	pin := repair142Pin(3, 1, 6999)
	valid := map[string]any{"key_id": base64.StdEncoding.EncodeToString(pin.KeyId), "public_key": base64.StdEncoding.EncodeToString(pin.PublicKey), "first_height": uint64(1), "last_height": uint64(6999)}
	encoded, err := json.Marshal([]any{valid})
	require.NoError(t, err)
	repair142Write(t, source, encoded)
	repair142Command(t, "python3", "sequencer-pins.py", "--gateway", source, output)
	saved, err := os.ReadFile(output)
	require.NoError(t, err)
	cases := map[string][]byte{"empty": []byte("[]"), "legacy": []byte(`{"layerx":{}}`), "duplicate-id": append(append([]byte("["), encoded[1:len(encoded)-1]...), append([]byte(","), encoded[1:]...)...), "trailing": append(bytes.Clone(encoded), 'x'), "oversize": bytes.Repeat([]byte(" "), 256*1024+1), "duplicate-field": []byte(strings.Replace(string(encoded), `"first_height":1`, `"first_height":2,"first_height":1`, 1))}
	for name, field := range map[string]any{"first_height": 0, "last_height": 0, "public_key": "AA==", "key_id": base64.StdEncoding.EncodeToString(make([]byte, 32))} {
		entry := map[string]any{}
		for key, value := range valid {
			entry[key] = value
		}
		entry[name] = field
		cases[name], err = json.Marshal([]any{entry})
		require.NoError(t, err)
	}
	for name, value := range map[string]any{"boolean": true, "negative": -1, "float": 1.0, "string": "1", "overflow": json.Number("18446744073709551616")} {
		entry := map[string]any{}
		for key, field := range valid {
			entry[key] = field
		}
		if name == "float" {
			cases[name] = []byte(strings.Replace(string(encoded), `"first_height":1`, `"first_height":1.0`, 1))
			continue
		}
		entry["first_height"] = value
		cases[name], err = json.Marshal([]any{entry})
		require.NoError(t, err)
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			repair142Write(t, source, data)
			command := exec.Command("python3", "sequencer-pins.py", "--gateway", source, output)
			result, err := command.CombinedOutput()
			require.Error(t, err, "malformed native pins accepted: %s", result)
			retained, err := os.ReadFile(output)
			require.NoError(t, err)
			require.Equal(t, saved, retained, "failed update replaced the retained list")
		})
	}
}

func TestRepair_142_WebhookContract(t *testing.T) {
	output := repair142Command(t, "python3", "-m", "pytest", "-q", "-p", "no:cacheprovider", "test_sequencer_pins.py")
	require.Contains(t, string(output), "19 passed", "all original trust-history cases and the webhook projection must execute")
}

package hostedtests

import (
	"archive/tar"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	seqmodule "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/bech32"

	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type SourceSnapshot struct {
	Revision   string `json:"revision"`
	SourceTree string `json:"source_tree"`
	Dirty      bool   `json:"dirty"`
	Tag        string `json:"tag"`
}

func buildInput(name string) bool {
	for _, part := range strings.Split(name, "/") {
		if part == ".env" || strings.HasPrefix(part, ".env.") {
			return false
		}
	}
	return !strings.HasPrefix(name, "qual-logs/") && name != "NEEDS.md" && name != "STATUS.md"
}

func SnapshotSource(root string, archive io.Writer) (SourceSnapshot, error) {
	repository, err := git.PlainOpenWithOptions(root, &git.PlainOpenOptions{EnableDotGitCommonDir: true})
	if err != nil {
		return SourceSnapshot{}, err
	}
	head, err := repository.Head()
	if err != nil {
		return SourceSnapshot{}, err
	}
	commit, err := repository.CommitObject(head.Hash())
	if err != nil {
		return SourceSnapshot{}, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return SourceSnapshot{}, err
	}
	index, err := repository.Storer.Index()
	if err != nil {
		return SourceSnapshot{}, err
	}
	tracked := map[string]filemode.FileMode{}
	original := map[string]*object.File{}
	err = tree.Files().ForEach(func(file *object.File) error {
		if buildInput(file.Name) {
			tracked[file.Name] = file.Mode
			original[file.Name] = file
		}
		return nil
	})
	if err != nil {
		return SourceSnapshot{}, err
	}
	for _, entry := range index.Entries {
		if buildInput(entry.Name) {
			tracked[entry.Name] = entry.Mode
		}
	}
	names := make([]string, 0, len(tracked))
	for name := range tracked {
		names = append(names, name)
	}
	sort.Strings(names)
	digest := sha256.New()
	if archive == nil {
		archive = io.Discard
	}
	output := tar.NewWriter(io.MultiWriter(archive, digest))
	dirty := false
	for _, name := range names {
		filename := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Lstat(filename)
		if errors.Is(err, os.ErrNotExist) {
			dirty = true
			continue
		}
		if err != nil {
			return SourceSnapshot{}, err
		}
		var raw []byte
		var mode filemode.FileMode
		var link string
		if info.Mode()&os.ModeSymlink != 0 {
			mode = filemode.Symlink
			link, err = os.Readlink(filename)
			raw = []byte(link)
		} else if info.Mode().IsRegular() {
			mode = filemode.Regular
			if info.Mode()&0o111 != 0 {
				mode = filemode.Executable
			}
			raw, err = os.ReadFile(filename)
		} else {
			return SourceSnapshot{}, fmt.Errorf("unsupported tracked build input %s", name)
		}
		if err != nil {
			return SourceSnapshot{}, err
		}
		prior := original[name]
		if prior == nil || prior.Mode != mode || prior.Hash != plumbing.ComputeHash(plumbing.BlobObject, raw) {
			dirty = true
		}
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(raw)), Typeflag: tar.TypeReg, ModTime: time.Unix(0, 0)}
		if mode == filemode.Executable {
			header.Mode = 0o755
		}
		if mode == filemode.Symlink {
			header.Typeflag = tar.TypeSymlink
			header.Size = 0
			header.Linkname = link
			header.Mode = 0o777
		}
		if err := output.WriteHeader(header); err != nil {
			return SourceSnapshot{}, err
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := output.Write(raw); err != nil {
				return SourceSnapshot{}, err
			}
		}
	}
	if err := output.Close(); err != nil {
		return SourceSnapshot{}, err
	}
	snapshot := SourceSnapshot{Revision: head.Hash().String(), SourceTree: hex.EncodeToString(digest.Sum(nil)), Dirty: dirty}
	snapshot.Tag = snapshot.Revision[:12]
	if dirty {
		snapshot.Tag += "-dirty-" + snapshot.SourceTree[:16]
	}
	return snapshot, nil
}

type ImageBuildPlan struct {
	Name       string `json:"name"`
	Context    string `json:"context"`
	Dockerfile string `json:"dockerfile"`
	Archive    string `json:"archive"`
	SourceTree string `json:"source_tree"`
}

func PlanImage(name, dockerfile, context, prepared, archive string) (ImageBuildPlan, error) {
	plan := ImageBuildPlan{Name: name, Context: context, Dockerfile: dockerfile, Archive: archive}
	if path.IsAbs(dockerfile) || path.Clean(dockerfile) != dockerfile || strings.HasPrefix(dockerfile, "../") {
		return plan, errors.New("invalid image Dockerfile")
	}
	switch context {
	case ".":
	case "services/explorer/backend":
		if name != "explorer-backend" {
			return plan, errors.New("unexpected explorer context")
		}
		plan.Dockerfile = "Dockerfile"
	case "@registry-builder":
		if name != "platform-registry-builder" || prepared == "" {
			return plan, errors.New("registry context must be produced by build-env.sh --prepare-context")
		}
		plan.Context = prepared
		plan.Dockerfile = "Dockerfile"
	default:
		return plan, fmt.Errorf("undeclared image context %s", context)
	}
	return plan, nil
}

func ProjectImageContext(plan ImageBuildPlan, rootArchive io.Reader, destination io.Writer) (ImageBuildPlan, error) {
	digest := sha256.New()
	output := tar.NewWriter(io.MultiWriter(destination, digest))
	foundRecipe := false
	if plan.Name == "platform-registry-builder" {
		required := map[string]bool{"Dockerfile": false, "Dockerfile.dockerignore": false, "package.json": false, "package-lock.json": false, "rust-downloads.lock": false, "install-rust.sh": false, "cargo-config.toml": false, "layerx-rustc": false, "layerx-build": false}
		vendor := false
		err := filepath.WalkDir(plan.Context, func(filename string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("builder context contains nonregular input %s", filename)
			}
			relative, err := filepath.Rel(plan.Context, filename)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			raw, err := os.ReadFile(filename)
			if err != nil {
				return err
			}
			if _, ok := required[relative]; ok {
				required[relative] = true
			}
			if strings.HasPrefix(relative, "vendor/") {
				vendor = true
			}
			mode := int64(0o644)
			if info.Mode()&0o111 != 0 {
				mode = 0o755
			}
			if err := output.WriteHeader(&tar.Header{Name: relative, Mode: mode, Size: int64(len(raw)), Typeflag: tar.TypeReg, ModTime: time.Unix(0, 0)}); err != nil {
				return err
			}
			_, err = output.Write(raw)
			return err
		})
		if err != nil {
			return plan, err
		}
		for name, exists := range required {
			if !exists {
				return plan, fmt.Errorf("prepared registry context is missing %s", name)
			}
		}
		if !vendor {
			return plan, errors.New("prepared registry context is missing verified vendor inputs")
		}
		foundRecipe = true
	} else {
		input := tar.NewReader(rootArchive)
		for {
			header, err := input.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return plan, err
			}
			name := header.Name
			if plan.Context != "." {
				prefix := plan.Context + "/"
				switch {
				case strings.HasPrefix(name, prefix):
					name = strings.TrimPrefix(name, prefix)
				case name == "docker/explorer-backend/Dockerfile":
					name = "Dockerfile"
				case name == "docker/explorer-backend/Dockerfile.dockerignore":
					name = "Dockerfile.dockerignore"
				default:
					continue
				}
			}
			if name == plan.Dockerfile {
				foundRecipe = true
			}
			header.Name = name
			if err := output.WriteHeader(header); err != nil {
				return plan, err
			}
			if _, err := io.Copy(output, input); err != nil {
				return plan, err
			}
		}
	}
	if !foundRecipe {
		return plan, errors.New("image context does not contain its Dockerfile")
	}
	if err := output.Close(); err != nil {
		return plan, err
	}
	plan.SourceTree = hex.EncodeToString(digest.Sum(nil))
	return plan, nil
}

func SolanaPublicKey(document []byte) (string, error) {
	var values []json.RawMessage
	if err := json.Unmarshal(document, &values); err != nil || len(values) != ed25519.PrivateKeySize {
		return "", errors.New("mirror keypair must contain exactly 64 integer bytes")
	}
	raw := make([]byte, len(values))
	for i, value := range values {
		if err := json.Unmarshal(value, &raw[i]); err != nil || bytes.Equal(value, []byte("null")) {
			return "", errors.New("mirror keypair must contain strict integer bytes")
		}
	}
	derived := ed25519.NewKeyFromSeed(raw[:ed25519.SeedSize])
	if subtle.ConstantTimeCompare(derived, raw) != 1 {
		return "", errors.New("mirror public half does not match its signing seed")
	}
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	number := new(big.Int).SetBytes(derived[ed25519.SeedSize:])
	base, remainder := big.NewInt(58), new(big.Int)
	encoded := []byte{}
	for number.Sign() != 0 {
		number.QuoRem(number, base, remainder)
		encoded = append(encoded, alphabet[remainder.Int64()])
	}
	for _, b := range derived[ed25519.SeedSize:] {
		if b != 0 {
			break
		}
		encoded = append(encoded, '1')
	}
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}
	return string(encoded), nil
}

const NativeChainID = "hyperpax_125-1"

type TreasuryPolicy struct {
	Version   uint32 `json:"version"`
	NetworkID uint32 `json:"network_id"`
	AssetID   string `json:"asset_id"`
	Recipient string `json:"recipient"`
}

func (policy TreasuryPolicy) Validate() error {
	if policy.Version != 1 || policy.NetworkID != 125 {
		return errors.New("invalid native treasury policy domain")
	}
	for value, length := range map[string]int{policy.AssetID: 32, policy.Recipient: 20} {
		raw, err := hex.DecodeString(value)
		if err != nil || len(raw) != length || value != strings.ToLower(value) || bytes.Equal(raw, make([]byte, length)) {
			return errors.New("invalid native treasury policy binding")
		}
	}
	return nil
}

type NativeMaterial struct {
	Schema     string            `json:"schema"`
	ChainID    string            `json:"chain_id"`
	EVMChainID uint64            `json:"evm_chain_id"`
	Treasury   TreasuryPolicy    `json:"treasury"`
	Files      map[string]string `json:"files"`
	directory  string
	closed     bool
}

var nativeInputPaths = map[string]string{
	"genesis.json": "config/genesis.json", "config.toml": "config/config.toml", "app.toml": "config/app.toml",
	"priv_validator_key.json": "config/priv_validator_key.json", "node_key.json": "config/node_key.json",
	"priv_validator_state.json": "data/priv_validator_state.json", "receipt.key": "config/receipt.key",
}

func readNativeFile(filename string) ([]byte, error) {
	info, err := os.Lstat(filename)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64*1024*1024 || info.Mode().Perm()&0o022 != 0 {
		return nil, fmt.Errorf("unsafe native material %s", filename)
	}
	return os.ReadFile(filename)
}

func writeNativeFile(filename string, raw []byte) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func validateNativeInputs(files map[string][]byte, policy TreasuryPolicy) (seqkeeper.ReceiptKeys, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	var genesis struct {
		ChainID    string                     `json:"chain_id"`
		Validators []json.RawMessage          `json:"validators"`
		AppState   map[string]json.RawMessage `json:"app_state"`
	}
	if err := json.Unmarshal(files["genesis.json"], &genesis); err != nil {
		return nil, err
	}
	if genesis.ChainID != NativeChainID || len(genesis.Validators) == 0 || genesis.AppState["bank"] == nil || genesis.AppState["lxphost"] == nil {
		return nil, errors.New("native genesis must bind validators, bank and merged kernel on chain 125")
	}
	var state struct {
		Leaders    [][]byte              `json:"leaders"`
		Keys       seqkeeper.ReceiptKeys `json:"receipt_keys"`
		Activation uint64                `json:"activation_height"`
		LastSeq    uint64                `json:"last_seq"`
		LastHash   []byte                `json:"last_seq_hash"`
	}
	if err := json.Unmarshal(genesis.AppState["sequencer"], &state); err != nil {
		return nil, err
	}
	if len(state.Keys) == 0 {
		return nil, errors.New("native genesis must pin its receipt signing key")
	}
	if err := state.Keys.Validate(); err != nil {
		return nil, err
	}
	activation := seqtypes.GenesisState{ActivationHeight: state.Activation, LastSeq: state.LastSeq}
	if len(state.LastHash) != 0 && len(state.LastHash) != 32 {
		return nil, errors.New("invalid sequence hash")
	}
	copy(activation.LastSeqHash[:], state.LastHash)
	for _, leader := range state.Leaders {
		if len(leader) != 32 {
			return nil, errors.New("invalid native leader")
		}
		activation.Leaders = append(activation.Leaders, [32]byte(leader))
	}
	for _, key := range state.Keys {
		activation.ReceiptKeys = append(activation.ReceiptKeys, seqtypes.ReceiptKey{KeyId: [32]byte(key.KeyId), PublicKey: [32]byte(key.PublicKey), FirstHeight: key.FirstHeight, LastHeight: key.LastHeight})
	}
	if err := activation.Validate(); err != nil {
		return nil, err
	}
	seed, err := hex.DecodeString(strings.TrimSpace(string(files["receipt.key"])))
	if err != nil || len(seed) != 32 || bytes.Equal(seed, make([]byte, 32)) {
		return nil, errors.New("native receipt seed is invalid")
	}
	public := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	pinned := false
	for _, key := range state.Keys {
		if bytes.Equal(key.PublicKey, public) && key.FirstHeight == 1 {
			pinned = true
		}
	}
	if !pinned {
		return nil, errors.New("native signing seed is not pinned at genesis")
	}
	for name, required := range map[string][]string{"priv_validator_key.json": {"priv_key", "pub_key", "address"}, "node_key.json": {"priv_key"}, "priv_validator_state.json": {"height", "round", "step"}} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(files[name], &fields); err != nil {
			return nil, err
		}
		for _, field := range required {
			if len(fields[field]) == 0 || bytes.Equal(fields[field], []byte("null")) {
				return nil, fmt.Errorf("native %s lacks %s", name, field)
			}
		}
	}
	var validator privval.FilePVKey
	if err := json.Unmarshal(files["priv_validator_key.json"], &validator); err != nil {
		return nil, err
	}
	if bytes.Equal(validator.PubKey.Bytes(), make([]byte, 32)) || !bytes.Equal(validator.PubKey.Bytes(), validator.PrivKey.Public().Bytes()) || !bytes.Equal(validator.Address, validator.PubKey.Address()) {
		return nil, errors.New("native validator keypair differs")
	}
	doc, err := tmtypes.GenesisDocFromJSON(files["genesis.json"])
	if err != nil {
		return nil, err
	}
	validatorFound := false
	for _, entry := range doc.Validators {
		if bytes.Equal(entry.PubKey.Bytes(), validator.PubKey.Bytes()) {
			validatorFound = true
		}
	}
	if !validatorFound || !bytes.Equal(state.Leaders[0], validator.PubKey.Bytes()) {
		return nil, errors.New("native validator does not lead the configured genesis")
	}
	if err := (seqmodule.AppModuleBasic{}).ValidateGenesis(nil, nil, genesis.AppState["sequencer"]); err != nil {
		return nil, err
	}
	var node tmtypes.NodeKey
	if err := json.Unmarshal(files["node_key.json"], &node); err != nil {
		return nil, err
	}
	var signing privval.FilePVLastSignState
	if err := json.Unmarshal(files["priv_validator_state.json"], &signing); err != nil {
		return nil, err
	}
	if signing.Height < 0 || signing.Round < 0 || signing.Step < 0 || signing.Step > 3 || (signing.Height > 0 && (len(signing.Signature) == 0 || len(signing.SignBytes) == 0)) {
		return nil, errors.New("invalid retained native signing state")
	}
	var config struct {
		Mode      string `toml:"mode"`
		Data      string `toml:"db-dir"`
		Genesis   string `toml:"genesis-file"`
		Node      string `toml:"node-key-file"`
		Validator struct {
			Key     string `toml:"key-file"`
			State   string `toml:"state-file"`
			Address string `toml:"laddr"`
		} `toml:"priv-validator"`
	}
	if _, err := toml.Decode(string(files["config.toml"]), &config); err != nil {
		return nil, err
	}
	if config.Mode != "validator" || config.Data != "data" || config.Genesis != "config/genesis.json" || config.Node != "config/node_key.json" || config.Validator.Key != "config/priv_validator_key.json" || config.Validator.State != "data/priv_validator_state.json" || config.Validator.Address != "" {
		return nil, errors.New("native configuration escapes the provisioned validator home")
	}
	var appConfig map[string]any
	if _, err := toml.Decode(string(files["app.toml"]), &appConfig); err != nil || len(appConfig) == 0 {
		return nil, errors.New("invalid native application configuration")
	}
	var evm struct {
		Associations []struct {
			Eth     string `json:"eth_address"`
			Account string `json:"pax_address"`
		} `json:"address_associations"`
	}
	var bank struct {
		Balances []struct {
			Address string `json:"address"`
			Coins   []struct {
				Denom  string `json:"denom"`
				Amount string `json:"amount"`
			} `json:"coins"`
		} `json:"balances"`
	}
	if err := json.Unmarshal(genesis.AppState["evm"], &evm); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(genesis.AppState["bank"], &bank); err != nil {
		return nil, err
	}
	funded := false
	for _, association := range evm.Associations {
		if strings.ToLower(association.Eth) != "0x"+policy.Recipient {
			continue
		}
		for _, balance := range bank.Balances {
			if balance.Address != association.Account {
				continue
			}
			for _, coin := range balance.Coins {
				amount, ok := new(big.Int).SetString(coin.Amount, 10)
				if ok && amount.Sign() > 0 && coin.Denom == "uhpx" {
					funded = true
				}
			}
		}
	}
	if !funded {
		return nil, errors.New("native treasury recipient is not bound to a funded genesis account")
	}
	return state.Keys, nil
}

func SaveNativeMaterial(home, directory string, policy TreasuryPolicy) (*NativeMaterial, error) {
	files := map[string][]byte{}
	for name, relative := range nativeInputPaths {
		raw, err := readNativeFile(filepath.Join(home, filepath.FromSlash(relative)))
		if err != nil {
			return nil, err
		}
		files[name] = raw
	}
	pins, err := validateNativeInputs(files, policy)
	if err != nil {
		return nil, err
	}
	files["receipt-keys.json"], err = GatewayReceiptKeys(pins)
	if err != nil {
		return nil, err
	}
	files["treasury-policy.json"], err = json.Marshal(policy)
	if err != nil {
		return nil, err
	}
	material := &NativeMaterial{Schema: "layerx.beta.native-material.v1", ChainID: NativeChainID, EVMChainID: 125, Treasury: policy, Files: map[string]string{}, directory: directory}
	if err := os.Mkdir(directory, 0o700); err != nil {
		return nil, err
	}
	for name, raw := range files {
		if err := writeNativeFile(filepath.Join(directory, name), raw); err != nil {
			return nil, err
		}
		material.Files[name] = fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	raw, err := json.Marshal(material)
	if err != nil {
		return nil, err
	}
	if err := writeNativeFile(filepath.Join(directory, "manifest.json"), raw); err != nil {
		return nil, err
	}
	parent, err := os.Open(directory)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if err := parent.Sync(); err != nil {
		return nil, err
	}
	return material, nil
}

func OpenNativeMaterial(directory string, expected TreasuryPolicy) (*NativeMaterial, error) {
	raw, err := readNativeFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var material NativeMaterial
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&material); err != nil {
		return nil, err
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, errors.New("trailing retained native material")
	}
	if material.Schema != "layerx.beta.native-material.v1" || material.ChainID != NativeChainID || material.EVMChainID != 125 || material.Treasury != expected || len(material.Files) != len(nativeInputPaths)+2 {
		return nil, errors.New("retained native chain identity or treasury policy changed")
	}
	files := map[string][]byte{}
	for name := range nativeInputPaths {
		files[name] = nil
	}
	files["receipt-keys.json"], files["treasury-policy.json"] = nil, nil
	for name := range files {
		raw, err := readNativeFile(filepath.Join(directory, name))
		if err != nil {
			return nil, err
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != material.Files[name] {
			return nil, fmt.Errorf("retained native input changed: %s", name)
		}
		files[name] = raw
	}
	pins, err := validateNativeInputs(files, expected)
	if err != nil {
		return nil, err
	}
	projected, err := GatewayReceiptKeys(pins)
	if err != nil || !bytes.Equal(projected, files["receipt-keys.json"]) {
		return nil, errors.New("retained native receipt keys changed")
	}
	policy, err := json.Marshal(expected)
	if err != nil || !bytes.Equal(policy, files["treasury-policy.json"]) {
		return nil, errors.New("retained treasury policy changed")
	}
	material.directory = directory
	return &material, nil
}

func (material *NativeMaterial) Close() { material.closed = true }

func (material *NativeMaterial) Secret(namespace string) (map[string]any, error) {
	if material.closed {
		return nil, errors.New("native material is closed")
	}
	data := map[string][]byte{}
	for name := range material.Files {
		raw, err := readNativeFile(filepath.Join(material.directory, name))
		if err != nil {
			return nil, err
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != material.Files[name] {
			return nil, errors.New("native material changed before projection")
		}
		data[name] = raw
	}
	raw, err := readNativeFile(filepath.Join(material.directory, "manifest.json"))
	if err != nil {
		return nil, err
	}
	data["manifest.json"] = raw
	return map[string]any{"apiVersion": "v1", "kind": "Secret", "type": "Opaque", "metadata": map[string]string{"name": "layerx-native-inputs", "namespace": namespace}, "data": data}, nil
}

func GatewayReceiptKeys(keys seqkeeper.ReceiptKeys) ([]byte, error) {
	if len(keys) == 0 {
		return nil, errors.New("empty native receipt keys")
	}
	if err := keys.Validate(); err != nil {
		return nil, err
	}
	type entry struct {
		KeyID  []byte `json:"keyId"`
		Public []byte `json:"publicKey"`
		First  uint64 `json:"firstHeight"`
		Last   uint64 `json:"lastHeight"`
	}
	output := make([]entry, len(keys))
	for i, key := range keys {
		output[i] = entry{key.KeyId, key.PublicKey, key.FirstHeight, key.LastHeight}
	}
	return json.Marshal(output)
}

func LoadGatewayReceiptKeys(raw []byte) (seqkeeper.ReceiptKeys, error) {
	if len(raw) > 256*1024 {
		return nil, errors.New("receipt keys exceed gateway bound")
	}
	if err := uniqueJSONFields(json.NewDecoder(bytes.NewReader(raw))); err != nil {
		return nil, err
	}
	var entries []struct {
		KeyID  string `json:"keyId"`
		Public string `json:"publicKey"`
		First  uint64 `json:"firstHeight"`
		Last   uint64 `json:"lastHeight"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&entries); err != nil {
		return nil, err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, errors.New("trailing receipt keys")
	}
	keys := make(seqkeeper.ReceiptKeys, len(entries))
	for i, entry := range entries {
		id, err := base64.StdEncoding.DecodeString(entry.KeyID)
		if err != nil || base64.StdEncoding.EncodeToString(id) != entry.KeyID {
			return nil, errors.New("invalid canonical receipt key ID")
		}
		public, err := base64.StdEncoding.DecodeString(entry.Public)
		if err != nil || base64.StdEncoding.EncodeToString(public) != entry.Public {
			return nil, errors.New("invalid canonical receipt public key")
		}
		keys[i] = seqkeeper.ReceiptKeyEntry{KeyId: id, PublicKey: public, FirstHeight: entry.First, LastHeight: entry.Last}
	}
	if len(keys) == 0 {
		return nil, errors.New("empty receipt keys")
	}
	return keys, keys.Validate()
}

func ConfigureNativeHome(home, recipientAccount string, policy TreasuryPolicy) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	genesisPath := filepath.Join(home, "config/genesis.json")
	raw, err := readNativeFile(genesisPath)
	if err != nil {
		return err
	}
	var genesis map[string]json.RawMessage
	if err := json.Unmarshal(raw, &genesis); err != nil {
		return err
	}
	var chain string
	if err := json.Unmarshal(genesis["chain_id"], &chain); err != nil || chain != NativeChainID {
		return errors.New("native genesis chain identity differs")
	}
	var app map[string]json.RawMessage
	if err := json.Unmarshal(genesis["app_state"], &app); err != nil {
		return err
	}
	if app["bank"] == nil || app["lxphost"] == nil {
		return errors.New("paxd init did not produce bank and merged kernel genesis")
	}
	var validator struct {
		Public json.RawMessage `json:"pub_key"`
	}
	raw, err = readNativeFile(filepath.Join(home, "config/priv_validator_key.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &validator); err != nil {
		return err
	}
	var public struct {
		Value []byte `json:"value"`
	}
	if err := json.Unmarshal(validator.Public, &public); err != nil || len(public.Value) != 32 {
		return errors.New("native validator is not Ed25519")
	}
	seedRaw, err := readNativeFile(filepath.Join(home, "config/receipt.key"))
	if err != nil {
		return err
	}
	seed, err := hex.DecodeString(strings.TrimSpace(string(seedRaw)))
	if err != nil || len(seed) != 32 || bytes.Equal(seed, make([]byte, 32)) {
		return errors.New("native receipt key seed is invalid")
	}
	receiptPublic := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	id := sha256.Sum256(receiptPublic)
	keys := seqkeeper.ReceiptKeys{{KeyId: id[:], PublicKey: receiptPublic, FirstHeight: 1, LastHeight: ^uint64(0)}}
	state := struct {
		Leaders [][]byte              `json:"leaders"`
		Keys    seqkeeper.ReceiptKeys `json:"receipt_keys"`
	}{[][]byte{public.Value}, keys}
	app["sequencer"], err = json.Marshal(state)
	if err != nil {
		return err
	}
	var evm map[string]json.RawMessage
	if err := json.Unmarshal(app["evm"], &evm); err != nil {
		return err
	}
	prefix, account, err := bech32.DecodeAndConvert(recipientAccount)
	if err != nil || prefix != "pax" || len(account) != 20 {
		return errors.New("native treasury requires its funded Paxeer account")
	}
	var associations []map[string]string
	if prior := evm["address_associations"]; len(prior) != 0 {
		if err := json.Unmarshal(prior, &associations); err != nil {
			return err
		}
	}
	exists := false
	for _, association := range associations {
		if strings.EqualFold(association["eth_address"], "0x"+policy.Recipient) || association["pax_address"] == recipientAccount {
			if !strings.EqualFold(association["eth_address"], "0x"+policy.Recipient) || association["pax_address"] != recipientAccount {
				return errors.New("native treasury association conflicts with genesis")
			}
			exists = true
		}
	}
	if !exists {
		associations = append(associations, map[string]string{"eth_address": "0x" + policy.Recipient, "pax_address": recipientAccount})
	}
	evm["address_associations"], err = json.Marshal(associations)
	if err != nil {
		return err
	}
	app["evm"], err = json.Marshal(evm)
	if err != nil {
		return err
	}
	genesis["app_state"], err = json.Marshal(app)
	if err != nil {
		return err
	}
	genesis["validators"], err = json.Marshal([]map[string]any{{"pub_key": validator.Public, "power": "7000000000"}})
	if err != nil {
		return err
	}
	raw, err = json.Marshal(genesis)
	if err != nil {
		return err
	}
	if err := os.WriteFile(genesisPath, raw, 0o600); err != nil {
		return err
	}
	return nil
}

func uniqueJSONFields(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	seen := map[string]bool{}
	for decoder.More() {
		if delimiter == '{' {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate JSON field")
			}
			seen[name] = true
		}
		if err := uniqueJSONFields(decoder); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

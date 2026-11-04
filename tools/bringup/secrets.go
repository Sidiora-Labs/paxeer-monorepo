package bringup

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type MintDelivery struct {
	Service, Kind, Variable, Secret string
	File                            bool
}

// MintDeliveries names only current owners. Tokens are generated per trust
// boundary and shared solely with its producer and its receiving consumer.
func MintDeliveries() []MintDelivery {
	var out []MintDelivery
	add := func(service, kind, variable, secret string, file bool) {
		out = append(out, MintDelivery{service, kind, variable, secret, file})
	}
	for _, v := range []struct{ name, secret string }{{"TENANT", "tenant"}, {"KEY", "binding-key"}} {
		add("wallet-gateway", "railway", "WALLET_IDENTITY_BINDING_"+v.name, v.secret, v.name != "TENANT")
	}
	for _, service := range []string{"wallet-gateway", "paxeer-attestor-1", "paxeer-attestor-2", "paxeer-attestor-3", "paxeer-attestor-4", "paxeer-attestor-5"} {
		kind := "box"
		if service == "wallet-gateway" {
			kind = "railway"
		} else {
			add(service, kind, "WALLET_IDENTITY_BINDING_PUBLIC_KEY", "binding-public", true)
		}
		add(service, kind, "WALLET_CUSTODY_INVENTORY", "inventory", true)
		add(service, kind, "WALLET_CUSTODY_INVENTORY_PUBLIC_KEY", "inventory-public", true)
	}
	for _, v := range []struct{ variable, secret string }{{"EVENTS_PAYMENT_TOKEN", "payment"}, {"EVENTS_WEBHOOKS_TOKEN", "trigger"}, {"IDENTITY_TOKEN", "id-gateway"}, {"IDENTITY_PROVISIONING_TOKEN", "id-provisioning"}, {"PROGRAM_REGISTRY_TOKEN", "registry-request"}, {"KEY_PROVISIONING_KEY", "key-provisioning"}, {"MODULE_REGISTRY", "modules"}} {
		add("gateway", "railway", "ENDPOINT_"+v.variable, v.secret, true)
	}
	for _, v := range []struct{ variable, secret string }{{"JOURNEY_TOKEN", "journey"}, {"APPROVAL_TOKEN", "approval"}, {"WEBHOOKS_TOKEN", "trigger"}} {
		add("paxeer-human-service", "box", "HUMAN_EVENTS_"+v.variable, v.secret, true)
	}
	for _, v := range []struct{ variable, secret string }{{"IDENTITY_TOKEN", "id-registry"}, {"PROGRAM_EVENTS_TOKEN", "program"}, {"WEBHOOKS_EVENTS_TOKEN", "trigger"}, {"REQUEST_TOKEN", "registry-request"}, {"PUBLICATION_TOKEN", "registry-publication"}} {
		add("paxeer-program-registry", "box", "REGISTRY_"+v.variable, v.secret, false)
	}
	add("webhooks-public", "railway", "WEBHOOKS_IDENTITY_TOKEN", "id-webhooks", true)
	for _, service := range []string{"webhooks-ingress"} {
		add(service, "railway", "WEBHOOKS_KMS_TOKEN", "kms", true)
		add(service, "railway", "WEBHOOKS_CURSOR_KEY", "cursor", true)
	}
	for _, v := range []struct{ variable, secret string }{{"JOURNEY_SOURCE_TOKEN", "journey-source"}, {"PAYMENT_SOURCE_TOKEN", "payment-source"}, {"APPROVAL_SOURCE_TOKEN", "approval-source"}, {"PROGRAM_SOURCE_TOKEN", "program-source"}, {"SOURCE_TRIGGER_TOKEN", "trigger"}, {"OPERATOR_TOKEN", "operator"}} {
		add("webhooks-ingress", "railway", "WEBHOOKS_"+v.variable, v.secret, true)
	}
	add("internal-kms", "railway", "INTERNAL_KMS_TOKEN", "kms", true)
	add("internal-kms", "railway", "INTERNAL_KMS_SEAL_SECRET", "kms-seal", true)
	for _, v := range []struct{ role, stem, secret string }{{"journeys", "JOURNEYS", "journey"}, {"payments", "PAYMENTS", "payment"}, {"approvals", "APPROVALS", "approval"}, {"programs", "PROGRAMS", "program"}} {
		add("internal-"+v.role, "railway", "INTERNAL_"+v.stem+"_TOKEN", v.secret+"-source", true)
		add("internal-"+v.role, "railway", "INTERNAL_"+v.stem+"_PRODUCER_TOKEN", v.secret, true)
	}
	// identity/env-init.sh writes these env tokens verbatim (not base64).
	for _, service := range []string{"gateway", "registry", "webhooks", "dashboard", "faucet", "testnet", "ramp", "provisioning", "registrar"} {
		add("identity", "railway", "IDENTITY_"+strings.ToUpper(service)+"_TOKEN", "id-"+service, false)
	}
	add("dashboard", "railway", "DASHBOARD_IDENTITY_TOKEN", "id-dashboard", true)
	add("ramp", "railway", "RAMP_IDENTITY_TOKEN", "id-ramp", true)
	return out
}

type MintInputs struct {
	Issuer, Tenant  string
	Certificates    [][]byte
	Modules         []byte
	Now             time.Time
	RailwayServices map[string]string
}
type mintMember struct {
	ID  string `json:"id"`
	Pin string `json:"spki_sha256"`
}
type mintClaims struct {
	Version   int          `json:"version"`
	Issuer    string       `json:"iss"`
	Audience  string       `json:"aud"`
	Tenant    string       `json:"tenant"`
	Sequence  string       `json:"sequence"`
	Issued    int64        `json:"iat"`
	Expires   int64        `json:"exp"`
	Protocol  string       `json:"protocol"`
	Threshold int          `json:"threshold"`
	Members   []mintMember `json:"members"`
	Keys      []any        `json:"keys"`
}

func mintPins(certificates [][]byte, now time.Time) ([]mintMember, error) {
	if len(certificates) != 5 {
		return nil, errors.New("five attestor certificates required")
	}
	var members []mintMember
	seen := map[string]bool{}
	for i, raw := range certificates {
		cert, e := parsePEMCertificate(raw)
		if e != nil || now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) || cert.IsCA {
			return nil, errors.New("attestor certificate malformed, expired or not a leaf")
		}
		sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
		pin := hex.EncodeToString(sum[:])
		if seen[pin] {
			return nil, errors.New("attestor identities must have distinct keys")
		}
		seen[pin] = true
		members = append(members, mintMember{strconvMint(i + 1), pin})
	}
	return members, nil
}
func strconvMint(i int) string { return fmt.Sprintf("%d", i) }
func validateMintInputs(input MintInputs) ([]mintMember, error) {
	if !strings.HasPrefix(input.Issuer, "https://") || !strings.HasSuffix(input.Issuer, "/auth/v1") || strings.ContainsAny(input.Issuer, "\r\n\t ") || !safeIdentity(input.Tenant) {
		return nil, errors.New("attestor authority identity refused")
	}
	var modules struct {
		Version int `json:"schema_version"`
		Assets  []struct {
			Asset, Symbol, Currency string
			Decimals                int
		}
		Modules []struct {
			Module   int
			Ordinals []int
		}
	}
	if json.Unmarshal(input.Modules, &modules) != nil || modules.Version != 2 || len(modules.Assets) == 0 || len(modules.Modules) == 0 {
		return nil, errors.New("module registry malformed")
	}
	seen := map[string]bool{}
	for _, a := range modules.Assets {
		if !fleetSHA.MatchString(a.Asset) || a.Symbol == "" || a.Currency == "" || a.Decimals < 0 || a.Decimals > 38 || seen[a.Asset] {
			return nil, errors.New("module registry asset refused")
		}
		seen[a.Asset] = true
	}
	modulesSeen := map[int]bool{}
	for _, module := range modules.Modules {
		if module.Module <= 0 || modulesSeen[module.Module] || len(module.Ordinals) == 0 {
			return nil, errors.New("module registry module refused")
		}
		modulesSeen[module.Module] = true
		ordinals := map[int]bool{}
		for _, ordinal := range module.Ordinals {
			if ordinal <= 0 || ordinals[ordinal] {
				return nil, errors.New("module registry ordinal refused")
			}
			ordinals[ordinal] = true
		}
	}
	return mintPins(input.Certificates, input.Now)
}
func pemMintKey(key *ecdsa.PrivateKey) ([]byte, []byte, error) {
	raw, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		return nil, nil, e
	}
	pub, e := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if e != nil {
		return nil, nil, e
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}), nil
}
func mintInventory(key *ecdsa.PrivateKey, input MintInputs, members []mintMember) ([]byte, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","typ":"wallet-custody-inventory+jwt"}`))
	body, e := json.Marshal(mintClaims{1, input.Issuer, "wallet-custody-inventory", input.Tenant, "1", input.Now.Unix() - 60, input.Now.Unix() - 60 + 2592000, "wallet", 3, members, []any{}})
	if e != nil {
		return nil, e
	}
	message := header + "." + base64.RawURLEncoding.EncodeToString(body)
	sum := sha256.Sum256([]byte(message))
	r, s, e := ecdsa.Sign(rand.Reader, key, sum[:])
	if e != nil {
		return nil, e
	}
	signature := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	return []byte(message + "." + base64.RawURLEncoding.EncodeToString(signature) + "\n"), nil
}
func MintSecrets(directory string, input MintInputs) ([]string, error) {
	members, e := validateMintInputs(input)
	if e != nil {
		return nil, e
	}
	if _, e = os.Lstat(directory); !errors.Is(e, os.ErrNotExist) {
		return nil, errors.New("existing mint output refused")
	}
	stage, e := os.MkdirTemp(filepath.Dir(directory), ".mint-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(stage)
	if e = os.Mkdir(filepath.Join(stage, "keys"), 0700); e != nil {
		return nil, e
	}
	binding, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return nil, e
	}
	bindingPEM, bindingPublic, e := pemMintKey(binding)
	if e != nil {
		return nil, e
	}
	signer, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return nil, e
	}
	signerPEM, signerPublic, e := pemMintKey(signer)
	if e != nil {
		return nil, e
	}
	inventory, e := mintInventory(signer, input, members)
	if e != nil {
		return nil, e
	}
	material := map[string][]byte{"tenant": []byte(input.Tenant), "binding-key": bindingPEM, "binding-public": bindingPublic, "inventory": inventory, "inventory-public": signerPublic, "modules": input.Modules}
	for _, d := range MintDeliveries() {
		if _, ok := material[d.Secret]; !ok {
			raw := make([]byte, 32)
			if _, e = rand.Read(raw); e != nil {
				return nil, e
			}
			material[d.Secret] = []byte(hex.EncodeToString(raw))
		}
	}
	keys := map[string][]byte{"wallet_identity_binding_key.pem": bindingPEM, "wallet_identity_binding_public.pem": bindingPublic, "wallet_custody_inventory_signer.pem": signerPEM, "wallet_custody_inventory_authority.pem": signerPublic, "wallet_custody_inventory.jwt": inventory}
	for name, raw := range keys {
		if e = edgeAtomic(filepath.Join(stage, "keys", name), raw, 0600); e != nil {
			return nil, e
		}
	}
	files := map[string]map[string]string{}
	services := map[string]bool{}
	for _, d := range MintDeliveries() {
		suffix := ".box.env"
		if d.Kind == "railway" {
			suffix = ".railway.env"
			services[d.Service] = true
		}
		file := d.Service + suffix
		if files[file] == nil {
			files[file] = map[string]string{}
		}
		value := string(material[d.Secret])
		if d.File {
			value = base64.StdEncoding.EncodeToString(material[d.Secret])
		}
		files[file][d.Variable] = value
	}
	var names []string
	for name, values := range files {
		var vars []string
		for variable := range values {
			vars = append(vars, variable)
		}
		sort.Strings(vars)
		var out strings.Builder
		for _, variable := range vars {
			fmt.Fprintln(&out, variable+"="+values[variable])
		}
		if e = edgeAtomic(filepath.Join(stage, name), []byte(out.String()), 0600); e != nil {
			return nil, e
		}
		names = append(names, name)
	}
	for service := range services {
		consumer := service
		if input.RailwayServices != nil {
			consumer = input.RailwayServices[service]
			if !safeIdentity(consumer) {
				return nil, errors.New("mint Railway consumer missing from rollout")
			}
		}
		name := service + ".railway.sh"
		script := "#!/bin/sh\nset -eu\nrailway=${LAYERX_RAILWAY_BIN:-$HOME/.railway/bin/railway}\nwhile IFS= read -r pair; do\n printf '%s' \"${pair#*=}\" | \"$railway\" variable set \"${pair%%=*}\" --stdin --service " + consumer + " --environment \"${LAYERX_RAILWAY_ENVIRONMENT:-beta}\" --skip-deploys >/dev/null\ndone <\"$(dirname \"$0\")/" + service + ".railway.env\"\n"
		if e = edgeAtomic(filepath.Join(stage, name), []byte(script), 0600); e != nil {
			return nil, e
		}
		names = append(names, name)
	}
	if e = edgeSyncDir(stage); e != nil {
		return nil, e
	}
	if _, e = os.Lstat(directory); !errors.Is(e, os.ErrNotExist) {
		return nil, errors.New("mint output appeared during publication")
	}
	if e = os.Rename(stage, directory); e != nil {
		return nil, e
	}
	if e = edgeSyncDir(filepath.Dir(directory)); e != nil {
		return nil, e
	}
	sort.Strings(names)
	for i := range names {
		names[i] = filepath.Join(directory, names[i])
	}
	return names, nil
}

// ReadMintEnvironment is the delivery consumer used by mint-check. It proves
// the exact service's closed schema before decoding file secrets as the
// image shim does; raw EnvironmentFile tokens stay raw.
func ReadMintEnvironment(service string, r io.Reader) (map[string][]byte, error) {
	deliveries := map[string]MintDelivery{}
	for _, d := range MintDeliveries() {
		if d.Service == service {
			deliveries[d.Variable] = d
		}
	}
	if len(deliveries) == 0 {
		return nil, errors.New("secret consumer is not declared")
	}
	values := map[string][]byte{}
	scan := bufio.NewScanner(io.LimitReader(r, 8<<20))
	scan.Buffer(make([]byte, 4096), 1<<20)
	for scan.Scan() {
		name, value, ok := strings.Cut(scan.Text(), "=")
		d, exists := deliveries[name]
		if !ok || !exists || value == "" || strings.ContainsAny(value, "\r\n\t ") || values[name] != nil {
			return nil, errors.New("secret environment malformed or foreign")
		}
		raw := []byte(value)
		if d.File {
			var e error
			raw, e = base64.StdEncoding.Strict().DecodeString(value)
			if e != nil || len(raw) == 0 {
				return nil, errors.New("secret file encoding refused")
			}
		}
		if d.Secret != "tenant" && d.Secret != "binding-key" && d.Secret != "binding-public" && d.Secret != "inventory" && d.Secret != "inventory-public" && d.Secret != "modules" && !fleetSHA.Match(raw) {
			return nil, errors.New("consumer token is not hexadecimal")
		}
		values[name] = raw
	}
	if e := scan.Err(); e != nil {
		return nil, e
	}
	if len(values) != len(deliveries) {
		return nil, errors.New("secret consumer missing a value")
	}
	return values, nil
}
func VerifyMintInventory(token, public []byte, certificates [][]byte, issuer, tenant string, now time.Time) error {
	block, rest := pem.Decode(public)
	if block == nil || block.Type != "PUBLIC KEY" || len(bytes.TrimSpace(rest)) != 0 {
		return errors.New("inventory authority malformed")
	}
	key, e := x509.ParsePKIXPublicKey(block.Bytes)
	if e != nil {
		return e
	}
	ec, ok := key.(*ecdsa.PublicKey)
	if !ok || ec.Curve != elliptic.P256() {
		return errors.New("inventory authority is not P-256")
	}
	parts := strings.Split(strings.TrimSpace(string(token)), ".")
	if len(parts) != 3 {
		return errors.New("inventory JWT malformed")
	}
	header, e := base64.RawURLEncoding.Strict().DecodeString(parts[0])
	if e != nil || string(header) != `{"alg":"ES256","typ":"wallet-custody-inventory+jwt"}` {
		return errors.New("inventory JWT header refused")
	}
	signature, e := base64.RawURLEncoding.Strict().DecodeString(parts[2])
	if e != nil || len(signature) != 64 {
		return errors.New("inventory signature malformed")
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(ec, sum[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])) {
		return errors.New("inventory signature refused")
	}
	body, e := base64.RawURLEncoding.Strict().DecodeString(parts[1])
	if e != nil {
		return e
	}
	var claims mintClaims
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if e = d.Decode(&claims); e != nil {
		return e
	}
	members, e := mintPins(certificates, now)
	if e != nil {
		return e
	}
	expected, _ := json.Marshal(members)
	actual, _ := json.Marshal(claims.Members)
	if claims.Version != 1 || claims.Issuer != issuer || claims.Tenant != tenant || claims.Audience != "wallet-custody-inventory" || claims.Protocol != "wallet" || claims.Threshold != 3 || claims.Sequence != "1" || claims.Issued > now.Unix() || claims.Expires <= now.Unix() || claims.Expires-claims.Issued != 2592000 || len(claims.Keys) != 0 || !bytes.Equal(expected, actual) {
		return errors.New("inventory identity, lifetime or certificate pins differ")
	}
	return nil
}

// MintFileDestinations consumes the real four-column env-files table. Missing
// TLS/config inputs remain the deployment owner's responsibility; only values
// actually minted here are projected, and each must have a declared consumer.
func MintFileDestinations(values map[string][]byte, table io.Reader, role string) (map[string][]byte, error) {
	paths := map[string][]byte{}
	seen := map[string]bool{}
	s := bufio.NewScanner(table)
	for s.Scan() {
		line := s.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 4 || !filepath.IsAbs(f[1]) || filepath.Clean(f[1]) != f[1] {
			return nil, errors.New("file delivery table malformed")
		}
		applies := false
		for _, r := range strings.Split(f[3], ",") {
			if r == "*" || r == role {
				applies = true
			}
		}
		if !applies {
			continue
		}
		if raw, ok := values[f[0]]; ok {
			if f[2] != "" && f[2] != "0600" {
				return nil, errors.New("secret delivery mode differs")
			}
			if paths[f[1]] != nil {
				return nil, errors.New("duplicate secret file destination")
			}
			paths[f[1]] = raw
			seen[f[0]] = true
		}
	}
	if e := s.Err(); e != nil {
		return nil, e
	}
	for name := range values {
		if !seen[name] {
			return nil, errors.New("minted file secret has no consumer: " + name)
		}
	}
	return paths, nil
}

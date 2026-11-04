package bringup

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type SecretTarget struct {
	Role, Target, Consumer string
	Variables              map[string]string
}
type SecretPlan struct {
	Version int
	Targets []SecretTarget
}
type secretSource struct {
	File, Variable string
	Base64         bool
}

var survivingSecretSources = map[string]secretSource{
	"journey": {"paxeer-human-service.box.env", "HUMAN_EVENTS_JOURNEY_TOKEN", true}, "approval": {"paxeer-human-service.box.env", "HUMAN_EVENTS_APPROVAL_TOKEN", true}, "payment": {"router.railway.env", "ENDPOINT_EVENTS_PAYMENT_TOKEN", true}, "program": {"paxeer-program-registry.box.env", "REGISTRY_PROGRAM_EVENTS_TOKEN", false}, "trigger": {"webhooks-ingress.railway.env", "WEBHOOKS_SOURCE_TRIGGER_TOKEN", true},
	"kms": {"internal-kms.railway.env", "INTERNAL_KMS_TOKEN", true}, "seal": {"internal-kms.railway.env", "INTERNAL_KMS_SEAL_SECRET", true},
	"journey-source": {"internal-journeys.railway.env", "INTERNAL_JOURNEYS_TOKEN", true}, "approval-source": {"internal-approvals.railway.env", "INTERNAL_APPROVALS_TOKEN", true}, "payment-source": {"internal-payments.railway.env", "INTERNAL_PAYMENTS_TOKEN", true}, "program-source": {"internal-programs.railway.env", "INTERNAL_PROGRAMS_TOKEN", true},
	"registry-request": {"paxeer-program-registry.box.env", "REGISTRY_REQUEST_TOKEN", false}, "registry-publication": {"paxeer-program-registry.box.env", "REGISTRY_PUBLICATION_TOKEN", false},
	"key-provisioning": {"router.railway.env", "ENDPOINT_KEY_PROVISIONING_KEY", true}, "operator": {"webhooks-ingress.railway.env", "WEBHOOKS_OPERATOR_TOKEN", true}, "cursor": {"webhooks-ingress.railway.env", "WEBHOOKS_CURSOR_KEY", true},
}

func NewSecretPlan(rollout io.Reader, human io.Reader, moduleRegistry []byte, retained string) (SecretPlan, error) {
	var plan SecretPlan
	var rows struct {
		Service map[string]struct {
			Target, Image string
			Service       string `toml:"railway_service"`
			BoxRole       string `toml:"box_role"`
		}
	}
	if _, e := toml.NewDecoder(rollout).Decode(&rows); e != nil {
		return plan, e
	}
	var box struct{ App string }
	if _, e := toml.NewDecoder(human).Decode(&box); e != nil || !safeIdentity(box.App) {
		return plan, errors.New("Human box consumer absent")
	}
	if !json.Valid(moduleRegistry) {
		return plan, errors.New("module registry is not JSON")
	}
	gateway := ""
	for _, row := range rows.Service {
		if row.Image == "layerx-gateway" && row.Target == "railway" {
			if gateway != "" || !safeIdentity(row.Service) {
				return plan, errors.New("ambiguous gateway rollout")
			}
			gateway = row.Service
		}
	}
	if gateway == "" {
		return plan, errors.New("gateway rollout absent")
	}
	registry, ok := rows.Service["program-registry"]
	if !ok || registry.Target != "box" || registry.BoxRole != "program-registry" {
		return plan, errors.New("registry box rollout differs")
	}
	tokens := map[string]string{}
	sources := map[string]secretSource{}
	for key, value := range survivingSecretSources {
		sources[key] = value
	}
	for _, role := range []string{"gateway", "registry", "webhooks", "dashboard", "faucet", "testnet", "ramp", "provisioning", "registrar"} {
		sources["id-"+role] = secretSource{"identity.railway.env", "IDENTITY_" + strings.ToUpper(role) + "_TOKEN", true}
	}
	for label, source := range sources {
		var token string
		if retained != "" {
			value, e := ReadMintSecret(filepath.Join(retained, source.File), source.Variable, source.Base64)
			if e != nil {
				return plan, e
			}
			token = value
		} else {
			var b [32]byte
			if _, e := rand.Read(b[:]); e != nil {
				return plan, e
			}
			token = hex.EncodeToString(b[:])
		}
		for _, existing := range tokens {
			if token == existing {
				return plan, errors.New("independent secret capabilities collide")
			}
		}
		tokens[label] = token
	}
	add := func(role, target, consumer string, pairs ...string) {
		vars := map[string]string{}
		for _, pair := range pairs {
			key, label, _ := strings.Cut(pair, ":")
			token := tokens[label]
			if target == "railway" || strings.HasPrefix(key, "HUMAN_") {
				token = base64.StdEncoding.EncodeToString([]byte(token))
			}
			vars[key] = token
		}
		plan.Targets = append(plan.Targets, SecretTarget{role, target, consumer, vars})
	}
	add(box.App, "box", box.App, "HUMAN_EVENTS_JOURNEY_TOKEN:journey", "HUMAN_EVENTS_APPROVAL_TOKEN:approval", "HUMAN_EVENTS_WEBHOOKS_TOKEN:trigger")
	add("paxeer-program-registry", "box", registry.BoxRole, "REGISTRY_IDENTITY_TOKEN:id-registry", "REGISTRY_PROGRAM_EVENTS_TOKEN:program", "REGISTRY_WEBHOOKS_EVENTS_TOKEN:trigger", "REGISTRY_REQUEST_TOKEN:registry-request", "REGISTRY_PUBLICATION_TOKEN:registry-publication")
	add("gateway", "railway", gateway, "ENDPOINT_EVENTS_PAYMENT_TOKEN:payment", "ENDPOINT_EVENTS_WEBHOOKS_TOKEN:trigger", "ENDPOINT_IDENTITY_TOKEN:id-gateway", "ENDPOINT_IDENTITY_PROVISIONING_TOKEN:id-provisioning", "ENDPOINT_PROGRAM_REGISTRY_TOKEN:registry-request", "ENDPOINT_KEY_PROVISIONING_KEY:key-provisioning")
	plan.Targets[len(plan.Targets)-1].Variables["ENDPOINT_MODULE_REGISTRY"] = base64.StdEncoding.EncodeToString(moduleRegistry)
	add("webhooks-public", "railway", "webhooks-public", "WEBHOOKS_IDENTITY_TOKEN:id-webhooks")
	add("webhooks-ingress", "railway", "webhooks-ingress", "WEBHOOKS_KMS_TOKEN:kms", "WEBHOOKS_JOURNEY_SOURCE_TOKEN:journey-source", "WEBHOOKS_PAYMENT_SOURCE_TOKEN:payment-source", "WEBHOOKS_APPROVAL_SOURCE_TOKEN:approval-source", "WEBHOOKS_PROGRAM_SOURCE_TOKEN:program-source", "WEBHOOKS_SOURCE_TRIGGER_TOKEN:trigger", "WEBHOOKS_OPERATOR_TOKEN:operator", "WEBHOOKS_CURSOR_KEY:cursor")
	add("internal-kms", "railway", "internal-kms", "INTERNAL_KMS_TOKEN:kms", "INTERNAL_KMS_SEAL_SECRET:seal")
	for _, entry := range []struct{ Role, Label string }{{"journeys", "journey"}, {"payments", "payment"}, {"approvals", "approval"}, {"programs", "program"}} {
		prefix := "INTERNAL_" + strings.ToUpper(entry.Role)
		add("internal-"+entry.Role, "railway", "internal-"+entry.Role, prefix+"_TOKEN:"+entry.Label+"-source", prefix+"_PRODUCER_TOKEN:"+entry.Label)
	}
	identity := []string{}
	for _, role := range []string{"gateway", "registry", "webhooks", "dashboard", "faucet", "testnet", "ramp", "provisioning", "registrar"} {
		identity = append(identity, "IDENTITY_"+strings.ToUpper(role)+"_TOKEN:id-"+role)
	}
	add("identity", "railway", "identity", identity...)
	add("dashboard", "railway", "dashboard", "DASHBOARD_IDENTITY_TOKEN:id-dashboard")
	add("ramp", "railway", "ramp", "RAMP_IDENTITY_TOKEN:id-ramp")
	for _, target := range plan.Targets {
		if target.Target == "railway" && target.Role != "gateway" {
			row, ok := rows.Service[target.Role]
			if !ok || row.Target != "railway" || row.Service != target.Consumer {
				return SecretPlan{}, errors.New("secret target differs from current rollout")
			}
		}
	}
	plan.Version = 1
	return plan, nil
}

func ReadMintSecret(path, variable string, encoded bool) (string, error) {
	info, e := os.Lstat(path)
	if e != nil {
		return "", e
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return "", errors.New("secret source must be a protected regular file")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	value := ""
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		key, v, ok := strings.Cut(line, "=")
		if !ok {
			return "", errors.New("malformed secret environment")
		}
		if key == variable {
			if found {
				return "", errors.New("duplicate secret variable")
			}
			found = true
			value = v
		}
	}
	if !found {
		return "", errors.New("surviving secret missing from retained mint")
	}
	if encoded {
		b, e := base64.StdEncoding.Strict().DecodeString(value)
		if e != nil {
			return "", errors.New("invalid secret encoding")
		}
		value = string(b)
	}
	decoded, e := hex.DecodeString(value)
	if e != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != value {
		return "", errors.New("secret must be canonical 32-byte hex")
	}
	return value, nil
}

func (p SecretPlan) Write(directory string, output io.Writer) error {
	info, e := os.Stat(directory)
	if e != nil {
		return e
	}
	if info.Mode().Perm() != 0700 {
		return errors.New("mint directory must be 0700")
	}
	// Refuse all conflicts before publishing any file; existing mints are never overwritten.
	for _, target := range p.Targets {
		suffixes := []string{".box.env"}
		if target.Target == "railway" {
			suffixes = []string{".railway.env", ".railway.sh"}
		}
		for _, suffix := range suffixes {
			if _, e := os.Lstat(filepath.Join(directory, target.Role+suffix)); !os.IsNotExist(e) {
				return errors.New("secret target exists; use explicit migration")
			}
		}
	}
	for _, target := range p.Targets {
		keys := []string{}
		for key := range target.Variables {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var text strings.Builder
		for _, key := range keys {
			fmt.Fprintf(&text, "%s=%s\n", key, target.Variables[key])
		}
		stem := filepath.Join(directory, target.Role+"."+target.Target)
		if e = edgeAtomic(stem+".env", []byte(text.String()), 0600); e != nil {
			return e
		}
		fmt.Fprintln(output, stem+".env")
		if target.Target == "railway" {
			script := fmt.Sprintf("#!/bin/sh\nset -eu\nrailway=${LAYERX_RAILWAY_BIN:-$HOME/.railway/bin/railway}\nwhile IFS= read -r pair; do\n printf '%%s' \"${pair#*=}\" | \"$railway\" variable set \"${pair%%%%=*}\" --stdin --service %s --environment \"${LAYERX_RAILWAY_ENVIRONMENT:-beta}\" --skip-deploys >/dev/null\ndone <\"$(dirname \"$0\")/%s.railway.env\"\n", target.Consumer, target.Role)
			if e = edgeAtomic(stem+".sh", []byte(script), 0600); e != nil {
				return e
			}
			fmt.Fprintln(output, stem+".sh")
		}
	}
	return nil
}

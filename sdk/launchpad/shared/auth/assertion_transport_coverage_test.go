package auth

import (
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"
)

func TestServiceAssertionRejectsMalformedTransportAndPrincipal(t *testing.T) {
	pki := testpki.New(t)
	leaf := pki.Leaf("issuer", "localhost")
	key := testpki.Ed25519(t)
	certificate, err := x509.ParseCertificate(leaf.Pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	transport := AssertionTransport{ActorAssertionHeader, ActorSignatureHeader}
	principal := IdentityPrincipal{AccountID: uuid.NewString(), SessionID: uuid.NewString(), Wallet: strings.ToLower(goldenAddr), AuthEpoch: 1, ProofAt: time.Now(), AccessExpiresAt: time.Now().Add(10 * time.Second)}
	issuer := ServiceAssertionIssuer{Issuer: "gateway", Audience: "account", Key: key.Private, Transport: transport}
	verifier := ServiceAssertionVerifier{Issuer: issuer.Issuer, Audience: issuer.Audience, Key: key.Private.Public().(ed25519.PublicKey), Transport: transport, PeerCertSHA256: leaf.SHA256}
	req := func() *http.Request {
		r := httptest.NewRequest("POST", "https://localhost/v1/action", nil)
		r.Header.Set("X-Request-ID", uuid.NewString())
		r.TLS = &tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{certificate}, VerifiedChains: [][]*x509.Certificate{{certificate}}}
		return r
	}
	for _, mode := range []string{"valid", "bad assertion", "missing signature", "bad signature encoding", "principal wallet", "duplicate roles", "invalid role", "principal session", "principal trace"} {
		t.Run(mode, func(t *testing.T) {
			r := req()
			if err := issuer.SignRequest(r, principal, []string{"owner"}, nil); err != nil {
				t.Fatal(err)
			}
			raw, err := base64.RawURLEncoding.DecodeString(r.Header.Get(ActorAssertionHeader))
			if err != nil {
				t.Fatal(err)
			}
			var a ServiceAssertion
			if err := json.Unmarshal(raw, &a); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "principal wallet":
				a.Principal.Wallet = "0x" + strings.Repeat("A", 40)
			case "duplicate roles":
				a.Principal.Roles = []string{"owner", "owner"}
			case "invalid role":
				a.Principal.Roles = []string{"invalid role"}
			case "principal session":
				a.Principal.SessionID = "other"
			case "principal trace":
				a.Principal.TraceID = uuid.NewString()
			}
			raw, err = json.Marshal(a)
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set(ActorAssertionHeader, base64.RawURLEncoding.EncodeToString(raw))
			r.Header.Set(ActorSignatureHeader, base64.RawURLEncoding.EncodeToString(ed25519.Sign(key.Private, raw)))
			switch mode {
			case "bad assertion":
				r.Header.Set(ActorAssertionHeader, base64.RawURLEncoding.EncodeToString([]byte("{")))
			case "missing signature":
				r.Header.Del(ActorSignatureHeader)
			case "bad signature encoding":
				r.Header.Set(ActorSignatureHeader, "$")
			}
			got, err := verifier.Authenticate(r, nil)
			if mode == "valid" {
				if err != nil || got.Wallet != principal.Wallet {
					t.Fatalf("valid assertion %+v %v", got, err)
				}
			} else if !errors.Is(err, ErrScopeDenied) || got.Wallet != "" {
				t.Fatalf("invalid transport admitted %+v %v", got, err)
			}
		})
	}
	for _, mode := range []string{"invalid identity", "wallet case", "role", "key", "transport", "value"} {
		r := req()
		identity, signer, roles := principal, issuer, []string{"owner"}
		switch mode {
		case "invalid identity":
			identity.ProofAt = time.Now().Add(time.Hour)
		case "wallet case":
			identity.Wallet = "0x" + strings.Repeat("A", 40)
		case "role":
			roles = []string{"bad role"}
		case "key":
			signer.Key = nil
		case "transport":
			signer.Transport.SignatureHeader = signer.Transport.AssertionHeader
		}
		var err error
		if mode == "value" {
			err = writeSignedAssertion(r, transport, key.Private, make(chan int))
		} else {
			err = signer.SignRequest(r, identity, roles, nil)
		}
		if err == nil {
			t.Fatalf("invalid %s assertion issued", mode)
		}
	}
	requestIssuer := ServiceRequestIssuer{Issuer: "consumer", Audience: "identity", Key: key.Private, Transport: transport}
	requestVerifier := ServiceRequestVerifier{Issuer: requestIssuer.Issuer, Audience: requestIssuer.Audience, Key: key.Private.Public().(ed25519.PublicKey), Transport: transport, PeerCertSHA256: leaf.SHA256}
	for _, mode := range []string{"valid", "pin", "missing assertion", "bad assertion", "malformed assertion", "missing signature", "bad signature", "binding", "signature"} {
		r := req()
		if err := requestIssuer.SignRequest(r, nil); err != nil {
			t.Fatal(err)
		}
		v := requestVerifier
		switch mode {
		case "pin":
			v.PeerCertSHA256 = strings.Repeat("0", 64)
		case "missing assertion":
			r.Header.Del(ActorAssertionHeader)
		case "bad assertion":
			r.Header.Set(ActorAssertionHeader, "$")
		case "malformed assertion":
			r.Header.Set(ActorAssertionHeader, base64.RawURLEncoding.EncodeToString([]byte("{")))
		case "missing signature":
			r.Header.Del(ActorSignatureHeader)
		case "bad signature":
			r.Header.Set(ActorSignatureHeader, "$")
		case "binding":
			r.Method = "PATCH"
		case "signature":
			r.Header.Set(ActorSignatureHeader, base64.RawURLEncoding.EncodeToString(make([]byte, 64)))
		}
		err := v.Authenticate(r, nil)
		if (mode == "valid" && err != nil) || (mode != "valid" && !errors.Is(err, ErrScopeDenied)) {
			t.Fatalf("request %s: %v", mode, err)
		}
	}
}

func TestReportedBlockDecoderRequiresCanonicalIdentity(t *testing.T) {
	header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(0), Time: 1, GasLimit: 1000000}
	wire, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(wire, &document); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"valid", "missing hash", "zero hash", "malformed hash", "invalid header", "null", "empty"} {
		candidate := map[string]any{}
		for k, v := range document {
			candidate[k] = v
		}
		switch mode {
		case "missing hash":
			delete(candidate, "hash")
		case "zero hash":
			candidate["hash"] = common.Hash{}.Hex()
		case "malformed hash":
			candidate["hash"] = "bad"
		case "invalid header":
			candidate["number"] = "bad"
		}
		raw, err := json.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		if mode == "null" {
			raw = []byte("null")
		}
		if mode == "empty" {
			raw = nil
		}
		block, err := decodeReportedBlock(raw)
		if mode == "valid" {
			if err != nil || block.Hash != header.Hash() || block.Number().Cmp(big.NewInt(1)) != 0 {
				t.Fatalf("reported block %+v %v", block, err)
			}
		} else if err == nil {
			t.Fatalf("invalid %s block admitted", mode)
		}
		if (mode == "empty" || mode == "null") && !errors.Is(err, ethereum.NotFound) {
			t.Fatalf("missing block: %v", err)
		}
	}
	if (ReportedBlock{}).Number() != nil {
		t.Fatal("absent header has number")
	}
}

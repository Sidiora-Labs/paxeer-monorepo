package bringup

// The identity producer uses OpenSSL's PBES2/AES-256-CBC PKCS#12 profile.
// Decode that profile in process so custody checks need no key-export tool.
import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"hash"
	"unicode/utf16"

	"golang.org/x/crypto/pbkdf2"
)

var (
	oidP12Data           = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	oidP12EncryptedData  = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 6}
	oidP12KeyBag         = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 10, 1, 1}
	oidP12ShroudedKeyBag = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 10, 1, 2}
	oidP12CertBag        = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 10, 1, 3}
	oidP12X509           = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 22, 1}
	oidPBES2             = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oidPBKDF2            = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}
	oidAES256CBC         = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}
	oidHMACSHA256        = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 9}
	oidSHA256            = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
)

type p12Content struct {
	Type    asn1.ObjectIdentifier
	Content asn1.RawValue `asn1:"optional,explicit,tag:0"`
}
type p12Digest struct {
	Algorithm pkix.AlgorithmIdentifier
	Digest    []byte
}
type p12Mac struct {
	Digest     p12Digest
	Salt       []byte
	Iterations int `asn1:"optional,default:1"`
}
type p12Document struct {
	Version  int
	AuthSafe p12Content
	Mac      p12Mac
}
type p12Bag struct {
	ID         asn1.ObjectIdentifier
	Value      asn1.RawValue   `asn1:"explicit,tag:0"`
	Attributes []asn1.RawValue `asn1:"optional,set"`
}

func unmarshalP12(data []byte, value any) error {
	rest, err := asn1.Unmarshal(data, value)
	if err != nil || len(rest) != 0 {
		return errors.New("invalid PKCS12 encoding")
	}
	return nil
}

func decodeIdentityP12(data []byte, password string) (crypto.PrivateKey, []*x509.Certificate, error) {
	fail := func() (crypto.PrivateKey, []*x509.Certificate, error) {
		return nil, nil, errors.New("inconsistent PKCS12 identity")
	}
	var doc p12Document
	if unmarshalP12(data, &doc) != nil || doc.Version != 3 || !doc.AuthSafe.Type.Equal(oidP12Data) || !doc.Mac.Digest.Algorithm.Algorithm.Equal(oidSHA256) || doc.Mac.Iterations < 1 || doc.Mac.Iterations > 1_000_000 || len(doc.Mac.Salt) < 8 || len(doc.Mac.Salt) > 64 {
		return fail()
	}
	var authenticated []byte
	if unmarshalP12(doc.AuthSafe.Content.Bytes, &authenticated) != nil {
		return fail()
	}
	macKey := p12KDF(password, doc.Mac.Salt, doc.Mac.Iterations)
	mac := hmac.New(sha256.New, macKey)
	mac.Write(authenticated)
	if !hmac.Equal(mac.Sum(nil), doc.Mac.Digest.Digest) {
		return fail()
	}
	var contents []p12Content
	if unmarshalP12(authenticated, &contents) != nil || len(contents) > 8 {
		return fail()
	}
	var key crypto.PrivateKey
	var certs []*x509.Certificate
	for _, content := range contents {
		var safe []byte
		switch {
		case content.Type.Equal(oidP12Data):
			if unmarshalP12(content.Content.Bytes, &safe) != nil {
				return fail()
			}
		case content.Type.Equal(oidP12EncryptedData):
			var encrypted struct {
				Version int
				Info    struct {
					Type      asn1.ObjectIdentifier
					Algorithm pkix.AlgorithmIdentifier
					Bytes     []byte `asn1:"tag:0"`
				}
			}
			if unmarshalP12(content.Content.Bytes, &encrypted) != nil || encrypted.Version != 0 || !encrypted.Info.Type.Equal(oidP12Data) {
				return fail()
			}
			var err error
			safe, err = decryptP12(encrypted.Info.Algorithm, encrypted.Info.Bytes, password)
			if err != nil {
				return fail()
			}
		default:
			return fail()
		}
		var bags []p12Bag
		if unmarshalP12(safe, &bags) != nil || len(bags) > 16 {
			return fail()
		}
		for _, bag := range bags {
			switch {
			case bag.ID.Equal(oidP12CertBag):
				var certBag struct {
					Type  asn1.ObjectIdentifier
					Value []byte `asn1:"explicit,tag:0"`
				}
				if unmarshalP12(bag.Value.Bytes, &certBag) != nil || !certBag.Type.Equal(oidP12X509) {
					return fail()
				}
				cert, err := x509.ParseCertificate(certBag.Value)
				if err != nil {
					return fail()
				}
				certs = append(certs, cert)
			case bag.ID.Equal(oidP12KeyBag), bag.ID.Equal(oidP12ShroudedKeyBag):
				if key != nil {
					return fail()
				}
				der := bag.Value.Bytes
				if bag.ID.Equal(oidP12ShroudedKeyBag) {
					var encrypted struct {
						Algorithm pkix.AlgorithmIdentifier
						Data      []byte
					}
					if unmarshalP12(der, &encrypted) != nil {
						return fail()
					}
					var err error
					der, err = decryptP12(encrypted.Algorithm, encrypted.Data, password)
					if err != nil {
						return fail()
					}
				}
				var err error
				key, err = x509.ParsePKCS8PrivateKey(der)
				if err != nil {
					return fail()
				}
			default:
				return fail()
			}
		}
	}
	if key == nil || len(certs) != 2 {
		return fail()
	}
	return key, certs, nil
}

func decryptP12(algorithm pkix.AlgorithmIdentifier, data []byte, password string) ([]byte, error) {
	failure := errors.New("unsupported or invalid PKCS12 encryption")
	if !algorithm.Algorithm.Equal(oidPBES2) {
		return nil, failure
	}
	var parameters struct {
		KDF        pkix.AlgorithmIdentifier
		Encryption pkix.AlgorithmIdentifier
	}
	if unmarshalP12(algorithm.Parameters.FullBytes, &parameters) != nil || !parameters.KDF.Algorithm.Equal(oidPBKDF2) || !parameters.Encryption.Algorithm.Equal(oidAES256CBC) {
		return nil, failure
	}
	var kdf struct {
		Salt       []byte
		Iterations int
		Length     int                      `asn1:"optional"`
		PRF        pkix.AlgorithmIdentifier `asn1:"optional"`
	}
	if unmarshalP12(parameters.KDF.Parameters.FullBytes, &kdf) != nil || kdf.Iterations < 1 || kdf.Iterations > 1_000_000 || len(kdf.Salt) < 8 || len(kdf.Salt) > 64 || (kdf.Length != 0 && kdf.Length != 32) {
		return nil, failure
	}
	var digest func() hash.Hash
	if kdf.PRF.Algorithm.Equal(oidHMACSHA256) {
		digest = sha256.New
	} else if len(kdf.PRF.Algorithm) == 0 {
		digest = sha1.New
	} else {
		return nil, failure
	}
	var iv []byte
	if unmarshalP12(parameters.Encryption.Parameters.FullBytes, &iv) != nil || len(iv) != aes.BlockSize || len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return nil, failure
	}
	block, err := aes.NewCipher(pbkdf2.Key([]byte(password), kdf.Salt, kdf.Iterations, 32, digest))
	if err != nil {
		return nil, failure
	}
	plain := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, data)
	padding := int(plain[len(plain)-1])
	if padding < 1 || padding > aes.BlockSize || !bytes.Equal(plain[len(plain)-padding:], bytes.Repeat([]byte{byte(padding)}, padding)) {
		return nil, failure
	}
	return plain[:len(plain)-padding], nil
}

// RFC 7292 appendix B. The producer uses SHA-256 MACs (u=32, v=64, ID=3).
func p12KDF(password string, salt []byte, iterations int) []byte {
	unicode := utf16.Encode([]rune(password))
	unicode = append(unicode, 0)
	pass := make([]byte, len(unicode)*2)
	for n, value := range unicode {
		pass[n*2] = byte(value >> 8)
		pass[n*2+1] = byte(value)
	}
	expand := func(data []byte) []byte {
		if len(data) == 0 {
			return nil
		}
		result := make([]byte, 64*((len(data)+63)/64))
		for n := range result {
			result[n] = data[n%len(data)]
		}
		return result
	}
	input := append(expand(salt), expand(pass)...)
	digest := sha256.New()
	digest.Write(bytes.Repeat([]byte{3}, 64))
	digest.Write(input)
	result := digest.Sum(nil)
	for n := 1; n < iterations; n++ {
		value := sha256.Sum256(result)
		result = value[:]
	}
	return result
}

# Security parameter choice

## Armored private keys

The bcrypt cost used for armored private keys defaults to 12. It is the
`BcryptSecurityParameter` variable in [`armor.go`](armor.go).
`EncryptArmorPrivKey` records envelope version `1`, the effective bcrypt
`cost`, the KDF name and a random 16-byte salt. Costs below bcrypt's minimum
of 4 normalize to its default of 10; costs above 16 are refused before
performing KDF work. `encryptPrivKey` passes the effective cost and salt to
`bcrypt.GenerateFromPassword`. The returned bcrypt hash
is hashed with SHA-256 to obtain the 32-byte key used by NaCl secretbox to
encrypt and authenticate the exported private key.

`UnarmorDecryptPrivKey` uses the recorded cost, independently of the current
`BcryptSecurityParameter`, so exports survive restarts with a different
default. Imports accept costs from 4 through 16 and reject missing or invalid
versioned parameters and unsupported versions before deriving a key.
Unversioned legacy exports always use the historical production cost of 12.
An old export made with a custom cost cannot be identified automatically;
restore it using the original software and configuration, then re-export it
in the versioned format.

## File keyring verifier

The `file` backend in [`keyring/keyring.go`](keyring/keyring.go) stores its
entries under `keyring-file` within the configured keyring directory.
`newRealPrompt` writes a salted bcrypt verifier for the keyring passphrase to
`keyring-file/keyhash` and checks subsequent passphrases with
`bcrypt.CompareHashAndPassword`.

The creation call passes `2` as its cost argument, but the effective cost is
**10**. The `github.com/tendermint/crypto` version pinned in
[`go.mod`](../../go.mod), `v0.0.0-20191022145703-50d29ede1e15`, defines
`MinCost = 4` and `DefaultCost = 10`; its
[`GenerateFromPassword` implementation][bcrypt-source] replaces any cost
below `MinCost` with `DefaultCost`. The verifier stores its effective cost
and salt, which are read during comparison. `BcryptSecurityParameter` does
not control this verifier. The current verifier format is unversioned;
increasing the armor export cost does not upgrade it. Migrating the verifier
in an active directory cannot upgrade an older stolen backup: its verifier
still permits offline guesses at its original work factor.

## Offline password guessing

An attacker who obtains an armored export can test passphrase guesses
offline by deriving a key and attempting authenticated decryption. An
attacker who obtains a copy or backup of `keyring-file`, including `keyhash`,
can instead test keyring passphrase guesses directly against that verifier
at cost 10. Neither attack requires continued access to the original
computer, and the prompt's three-attempt limit does not restrict offline
guessing.

Each increment of bcrypt's effective cost approximately doubles its work.
Cost 10 therefore takes roughly one quarter of the bcrypt work of cost 12
under comparable conditions. If the same passphrase protects an export and
the keyring, the copied verifier provides a cheaper way to discover that
passphrase; export cost 12 does not make a directory backup equally costly
to attack.

Protection depends on passphrase strength as well as the work factor. Use
long, difficult-to-guess passphrases and avoid reusing them between the
keyring and exports. Protect exported keys and keyring backups against
unauthorized copying; the derived key's 32-byte length does not give a weak
passphrase equivalent strength.

[bcrypt-source]: https://github.com/tendermint/crypto/blob/50d29ede1e15/bcrypt/bcrypt.go

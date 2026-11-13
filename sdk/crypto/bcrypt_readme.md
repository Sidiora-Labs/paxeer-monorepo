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
`newRealPrompt` in [`keyring/verifier.go`](keyring/verifier.go) writes a salted
bcrypt verifier for the keyring passphrase to `keyring-file/keyhash` and checks
subsequent passphrases with `bcrypt.CompareHashAndPassword`.

New verifiers use bcrypt cost **12**, a random 16-byte salt and a JSON envelope
with `version = 1`, `kdf = "bcrypt"` and the encoded `hash`. Imports accept
costs from 4 through 16. After a successful password check, a raw bcrypt verifier
or a verifier below cost 12 is replaced atomically with a versioned verifier
using a fresh salt and a cost of at least 12. Higher accepted costs are retained;
failed password checks do not update the file. `BcryptSecurityParameter` controls
armored exports separately and does not set this verifier's cost.

Updating the verifier does not re-encrypt the keyring entries or strengthen
copies of earlier backups. Preserve both the encrypted entries and `keyhash`
when backing up the file backend.

## Offline password guessing

An attacker who obtains an armored export can test passphrase guesses
offline by deriving a key and attempting authenticated decryption. An
attacker who obtains a copy or backup of `keyring-file`, including `keyhash`,
can instead test keyring passphrase guesses directly against that verifier
at its recorded cost. Neither attack requires continued access to the original
computer, and the prompt's three-attempt limit does not restrict offline
guessing.

Each increment of bcrypt's effective cost approximately doubles its work.
Cost 10 takes roughly one quarter of the bcrypt work of cost 12 under comparable
conditions. A raw verifier using cost 10 remains searchable at that cost in an
older backup, even after the active verifier is upgraded. The encrypted files
and armored exports are separate password-checking artifacts with their own
parameters.

Protection depends on passphrase strength as well as the work factor. Use
long, difficult-to-guess passphrases and avoid reusing them between the
keyring and exports. Protect exported keys and keyring backups against
unauthorized copying; the derived key's 32-byte length does not give a weak
passphrase equivalent strength.

// Package keyring provides the Keyring API for local, watch-only, multisig and
// Ledger accounts. New selects an operating-system or file secret backend;
// NewInMemory keeps transient keys which disappear when the instance is lost.
//
// New supports os (the operating system's credential store), file (encrypted
// files under keyring-file), kwallet (KDE Wallet), pass (the pass utility), test
// and memory. OS stores may remain unlocked for the user's session. The test
// backend stores keys insecurely using a fixed public password, never prompts,
// and must only be used for testing. NewInMemory and memory are not durable.
//
// Default software creation, signing and armored private-key import support
// secp256k1 and sr25519. Import rejects other algorithm labels and respects the
// configured supported algorithms. Watch-only SavePubKey accepts secp256k1,
// sr25519 and ed25519 public keys, deriving the label when empty and rejecting
// mismatches. Ed25519 is for validator public information, not wallet private
// keys. SaveMultisig stores multisig references. Ledger advertises secp256k1
// only; creation and signing require a configured device and a Ledger-enabled
// executable. Unsupported algorithms are rejected before contacting the device.
//
// The file backend caches its password while the instance is in use. Its
// keyhash verifier is versioned, uses bcrypt cost 12 with a fresh random salt,
// and accepts backup costs only in the bounded range 4 through 16. Legacy raw
// bcrypt verifiers (including the old effective cost 10) are upgraded atomically
// after successful password verification; existing higher costs are retained.
// Failed passwords never migrate the verifier. Backups must retain both the
// encrypted entries and keyhash. The backend's own encrypted-file KDF and any
// armored exports are separate password-checking artifacts; strengthening
// keyhash does not re-encrypt older files or strengthen old backups. Choose a
// strong password, restrict directory access, and protect or retire old backups.
//
// Creates and deletes journal their intent in the same secret backend before
// updating the name and address views. New replays an interrupted operation;
// access and mutation also retry recovery. PendingOperationError means storage
// must be repaired before recovery can finish. An identical create retry does
// not replace the original account. List returns healthy records alongside a
// structured CorruptionError identifying damaged entries. Callers must check
// its error before displaying output; MkAccKeysOutput can accept List's two
// results directly and propagates that error.
//
// Recovery assumes completed backend Set calls survive a process restart and
// one active keyring instance per persistent store. The backend does not promise
// atomic file replacement or fsync of every secret write, so a torn journal,
// media loss or power failure may require restoring a protected backup. A
// corrupted journal is reported and retained rather than discarded. The
// journal repairs interrupted name/address operations, not arbitrary damage.
package keyring

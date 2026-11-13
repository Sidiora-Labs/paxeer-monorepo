# Using the smartcard wallet

## Requirements

  * A USB smartcard reader
  * A keycard that supports the status app
  * PCSCD version 4.3 running on your system **Only version 4.3 is currently supported**

## Preparing the smartcard

  **WARNING: FOLLOWING THESE INSTRUCTIONS WILL DESTROY THE MASTER KEY ON YOUR CARD. ONLY PROCEED IF NO FUNDS ARE ASSOCIATED WITH THESE ACCOUNTS**

  You can use status' [keycard-cli](https://github.com/status-im/keycard-cli) and you should get _at least_ version 2.1.1 of their [smartcard application](https://github.com/status-im/status-keycard/releases/download/2.2.1/keycard_v2.2.1.cap)

  You also need to make sure that the PCSC daemon is running on your system.

  Then, you can install the application to the card by typing:

  ```
  keycard install -a keycard_v2.2.1.cap && keycard init
  ```

  At the end of this process, you will be provided with a PIN, a PUK and a pairing password. Write them down, you'll need them shortly.

  Start Clef with a trusted UI using `clef --stdio-ui`. The UI can list the
  connected wallets over that bidirectional channel:

  ```json
  {"jsonrpc":"2.0","method":"clef_listWallets","params":[],"id":1}
  ```

  Use the returned `keycard://` URL to open the wallet. Send the pairing
  password generated during card initialization, then repeat with the PIN:

  ```json
  {"jsonrpc":"2.0","method":"clef_openWallet","params":["keycard://044def09","pairing password"],"id":2}
  {"jsonrpc":"2.0","method":"clef_openWallet","params":["keycard://044def09","PIN number"],"id":3}
  ```

  An empty card reports `Empty, waiting for initialization`. The Go smartcard
  API provides [`(*Wallet).Initialize(seed []byte) error`](wallet.go) to install
  a key from a seed. Keep a backup of that seed. This method is available to
  Go integrations; Clef's UI API does not expose wallet initialization.

## Usage

  1. Start Clef with the trusted UI and use `clef_listWallets` to find the card URL.
  2. Use `clef_openWallet` with the pairing password and then the PIN.
  3. Derive and pin an account using the trusted UI channel:

  ```json
  {"jsonrpc":"2.0","method":"clef_deriveAccount","params":["keycard://044def09","m/44'/60'/0'/0/0",true],"id":4}
  ```

  4. Use `clef_listWallets` to inspect the card status and accounts. Configure
     Geth with `--signer "$HOME/.clef/clef.ipc"` to request signatures from Clef.

  These `clef_` methods belong to the trusted UI channel used with `--stdio-ui`.
  They are not methods of Clef's external HTTP or IPC API.

## Known issues

  * Starting geth with a valid card seems to make firefox crash.
  * PCSC version 4.4 should work, but is currently untested

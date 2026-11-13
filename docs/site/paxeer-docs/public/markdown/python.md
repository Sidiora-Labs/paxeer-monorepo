<!-- Source: https://docs.paxeer.app/python/ -->

# Python Quickstart

Connect to Paxeer with web3.py, read PAX balances and ERC-20 state, and inspect transaction receipts without a signing key.

Python is useful for application backends, data analysis, and operational scripts. This guide reads the Paxeer EVM through HTTP using web3.py. Every example is read-only and expects chain ID 125, which is the ID for `hyperpax_125-1`; against a local or other network, expect its own ID instead (unmapped chains default to 713714) and adjust the check below.

## Install in an isolated environment

These examples use the web3.py 7 API. Create a virtual environment and keep the resulting dependency versions with your project.

```
python3 -m venv .venv
source .venv/bin/activate
python -m pip install "web3>=7,<8"
```

On Windows PowerShell, activate with `.venv\Scripts\Activate.ps1`. Supply `PAXEER_RPC_URL` and `ACCOUNT_ADDRESS` through your shell or deployment environment.

## Read the chain and account

Save this as `read_paxeer.py`, then run `python read_paxeer.py`. It validates the endpoint, captures one block, and reads the account at that height.

```
import os
from web3 import Web3

w3 = Web3(Web3.HTTPProvider(
    os.environ["PAXEER_RPC_URL"],
    request_kwargs={"timeout": 30},
))
if not w3.is_connected():
    raise RuntimeError("The EVM RPC endpoint did not respond")
if w3.eth.chain_id != 125:
    raise RuntimeError("Expected Paxeer EVM chain ID 125")

account = Web3.to_checksum_address(os.environ["ACCOUNT_ADDRESS"])
height = w3.eth.block_number
balance = w3.eth.get_balance(account, block_identifier=height)
nonce = w3.eth.get_transaction_count(account, block_identifier=height)

print("Chain ID:", w3.eth.chain_id)
print("Block:", height)
print("Balance (PAX):", w3.from_wei(balance, "ether"))
print("Nonce:", nonce)
```

`from_wei(..., "ether")` names an 18-decimal unit conversion in web3.py. On Paxeer the native currency being displayed is PAX. Use decimal values or integer base units for amounts; avoid converting balances through floating-point numbers.

## Read an ERC-20 balance

Set `TOKEN_ADDRESS` and append this block to the script. The ABI is intentionally limited to the two methods being called.

```
from decimal import Decimal, localcontext

abi = [
    {
        "type": "function", "name": "decimals", "stateMutability": "view",
        "inputs": [], "outputs": [{"name": "", "type": "uint8"}],
    },
    {
        "type": "function", "name": "balanceOf", "stateMutability": "view",
        "inputs": [{"name": "owner", "type": "address"}],
        "outputs": [{"name": "", "type": "uint256"}],
    },
]
token_address = Web3.to_checksum_address(os.environ["TOKEN_ADDRESS"])
if not w3.eth.get_code(token_address, block_identifier=height):
    raise RuntimeError("No contract exists at TOKEN_ADDRESS at this block")
token = w3.eth.contract(address=token_address, abi=abi)
decimals = token.functions.decimals().call(block_identifier=height)
raw_balance = token.functions.balanceOf(account).call(block_identifier=height)
with localcontext() as context:
    context.prec = 100
    print("Token balance:", Decimal(raw_balance) / (Decimal(10) ** decimals))
```

Use the token’s own decimals rather than PAX’s decimals. A missing method or unexpected return encoding indicates an ABI mismatch; obtain the ABI from the contract build or its verified deployment record.

## Inspect a transaction receipt

Set `TX_HASH` to a transaction hash from the same network. Append this block to distinguish a successful execution from a revert or a transaction the endpoint has not indexed.

```
from web3.exceptions import TransactionNotFound

try:
    receipt = w3.eth.get_transaction_receipt(os.environ["TX_HASH"])
except TransactionNotFound:
    print("No receipt yet. Verify the network and transaction hash before retrying.")
else:
    print("Execution:", "success" if receipt["status"] == 1 else "reverted")
    print("Block:", receipt["blockNumber"])
    print("Gas used:", receipt["gasUsed"])
    print("Logs:", len(receipt["logs"]))
```

## Use the result in an application

- Show an unavailable state when reads time out and offer a retry.

- For repeated queries, preserve block numbers in your output so users can judge freshness.

- Use bounded event ranges for indexers, following the [querying guide](https://docs.paxeer.app/querying-evm).

- For transaction signing, add an explicit account-selection and transaction-review step.

References: [web3.py 7 quickstart](https://web3py.readthedocs.io/en/v7.14.0/quickstart.html) and [contract calls](https://web3py.readthedocs.io/en/v7.14.0/contracts.html).

**Paxeer source:** `Paxeer-X-Network/` — rpc/state.go (GetBalance and GetCode), rpc/tx.go (GetTransactionReceipt), and modules/evm/config/config.go (ChainIDMapping).

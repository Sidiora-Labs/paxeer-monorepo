// Generated from the reviewed chain125 deployment manifest by client-deployment.cjs.
// Code identity is attested separately; pending governance acceptance remains a release gate.
export const CORE_DEPLOYMENT = Object.freeze({
  "chainId": 125,
  "manifestSha256": "a35ff7c6f532d2f92e4ca59e4ef2cf0a105d551ae9ac3eba68b95f79a355df4a",
  "recordedStatus": "finalized",
  "router": "0x78FB06f525cD964388f2305A489035263E31C6e4",
  "quoter": "0x793006f4F321b8863F26DBD75b11bAa01DA181dc",
  "feesRouter": "0x05681dA6f611C2dfFD3E92611F6dE2d247647927",
  "opticalRegistry": "0x0FB5E510372Bd3EddFED16A5Dd766d4473CffcF0",
  "protocolConfig": "0xF3669eDc475Cb6E1F0d7635240ABD8e8FB7E20C6",
  "factory": "0x3993D004166738d1069B5C4e0a923f7B9D5D8830",
  "nft": "0xe8DcC844a8006dbf64f5Bb851bCad90292a6409B",
  "eventEmitter": "0x622E1bb90E0E2B6E6e5e9e2DbE1555389E8A3EF6",
  "feeAccumulator": "0x7b14808b72f61664cDb79D25E6d1587655571a66",
  "poolRegistry": "0x1dC357d2B7B850332a91F06A9E66634F46fC4Ba7",
  "poolBeacon": "0x0c60Ea33B1DD602A17E1EF1875eE912F3fad71BE",
  "roleIdentity": {
    "router": {
      "kind": "surface",
      "runtimeHash": "0x6302ff80f4168c2af23afbf3117c613edd66d288d120eee825f9ecadd43a392f"
    },
    "quoter": {
      "kind": "surface",
      "runtimeHash": "0x6302ff80f4168c2af23afbf3117c613edd66d288d120eee825f9ecadd43a392f"
    },
    "feesRouter": {
      "kind": "surface",
      "runtimeHash": "0x6302ff80f4168c2af23afbf3117c613edd66d288d120eee825f9ecadd43a392f"
    },
    "opticalRegistry": {
      "kind": "erc1967",
      "runtimeHash": "0xb2f39d5c380b45131e836d4beb5254d4e5c410d35935bf270e8492755f7da6f7",
      "implementationAddress": "0x2d9666F5CC900092Ceb7D3867b1fd7f6178C3F3E",
      "implementationHash": "0xadf625b36b1dd5241f679fa029e66aa980eb5968f46fe557e8ae864ab157e0e5"
    },
    "protocolConfig": {
      "kind": "surface",
      "runtimeHash": "0x6302ff80f4168c2af23afbf3117c613edd66d288d120eee825f9ecadd43a392f"
    },
    "factory": {
      "kind": "surface",
      "runtimeHash": "0x6302ff80f4168c2af23afbf3117c613edd66d288d120eee825f9ecadd43a392f"
    },
    "nft": {
      "kind": "surface",
      "runtimeHash": "0x6302ff80f4168c2af23afbf3117c613edd66d288d120eee825f9ecadd43a392f"
    },
    "eventEmitter": {
      "kind": "erc1967",
      "runtimeHash": "0xb2f39d5c380b45131e836d4beb5254d4e5c410d35935bf270e8492755f7da6f7",
      "implementationAddress": "0x6d48a32e313cc4F08acdeE1F39F7e6B56F03a1cB",
      "implementationHash": "0xd0dc6f1aa54811dc2b9ea13e4519b6b6992ba0a082ac55c2d9e312443b289c08"
    },
    "feeAccumulator": {
      "kind": "surface",
      "runtimeHash": "0x6302ff80f4168c2af23afbf3117c613edd66d288d120eee825f9ecadd43a392f"
    },
    "poolRegistry": {
      "kind": "surface",
      "runtimeHash": "0x6302ff80f4168c2af23afbf3117c613edd66d288d120eee825f9ecadd43a392f"
    },
    "poolBeacon": {
      "kind": "beacon",
      "runtimeHash": "0xf06aa3fb14a04a50dcfbb95510fd19bfddc057f95349e63115b8495223aca447",
      "implementationAddress": "0xd24b239df92CA233dF27676148FDe10aa51104Df",
      "implementationHash": "0xeeadddb17507c0b32fb9c1c76f66b33b4757efd3531d86ac6a61c96d81d8bde4"
    }
  },
  "usdl": "0x85fcd13735f4309833a503ee804ea32395851479",
  "nativeSymbol": "PAX",
  "nativeDecimals": 18,
  "usdlDecimals": 6,
  "creationFeeRaw": "1000000"
} as const);

export const OPTIONAL_CONTRACT_CAPABILITIES = Object.freeze({ metaAg: false, pecorOrders: false } as const);
export type OptionalContractCapability = keyof typeof OPTIONAL_CONTRACT_CAPABILITIES;

export function isActiveApprovalSpender(spender: string): boolean {
  return typeof spender === 'string' && /^0x[0-9a-fA-F]{40}$/.test(spender) && spender.toLowerCase() === CORE_DEPLOYMENT.router.toLowerCase();
}

export function assertActiveApprovalSpender(spender: string): void {
  if (!isActiveApprovalSpender(spender)) throw new Error('Approval spender is unavailable: no verified active deployment for chain 125.');
}

export function assertOptionalContractCapability(capability: OptionalContractCapability): never {
  throw new Error('Optional contract capability ' + capability + ' is unavailable: no verified active deployment for chain 125.');
}

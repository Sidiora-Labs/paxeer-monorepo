import { getAbiItem, parseAbi, toFunctionSelector, toFunctionSignature, type Address } from 'viem';
import { routerAbi, quoterAbi, protocolConfigAbi } from '../../sidiora-launchpad-v3/dist/abis.js';

export { routerAbi, quoterAbi, protocolConfigAbi };

export const DEPLOYMENT = Object.freeze({
  chainId: 125,
  router: '0x78FB06f525cD964388f2305A489035263E31C6e4' as Address,
  quoter: '0x793006f4F321b8863F26DBD75b11bAa01DA181dc' as Address,
  protocolConfig: '0xF3669eDc475Cb6E1F0d7635240ABD8e8FB7E20C6' as Address,
  usdl: '0x85fcd13735f4309833a503ee804ea32395851479' as Address,
  spender: '0x0000000000000000000000000000000000001017' as Address,
  creationFeeRaw: '1000000',
});

export const HOME_CHAIN_ABI = parseAbi([
  'function homeChainOf(address pool) view returns (uint64)',
]);
export const VIRTUAL_CHAIN_ID_BASE = 125000000;
export const QUOTE_TTL_MS = 30000;
export const SIMULATION_TTL_MS = 10000;
export const TRADE_DEADLINE_SECONDS = 120;

function routerCall(name: 'buy' | 'sell' | 'createMarket') {
  const item = getAbiItem({ abi: routerAbi, name });
  return Object.freeze({
    contract: DEPLOYMENT.router,
    method: toFunctionSignature(item),
    selector: toFunctionSelector(item),
  });
}

export const EXECUTION_CALLS = Object.freeze({
  buy: routerCall('buy'),
  sell: routerCall('sell'),
  launch: routerCall('createMarket'),
  approve: Object.freeze({
    method: 'approve(address,uint256)',
    selector: toFunctionSelector('approve(address,uint256)'),
    spender: DEPLOYMENT.spender,
  }),
});

import { agentInterfaceDescription } from '@kindle/agent-kit';
import { backendOrigins, BackendOriginError, walletGatewayApiBase } from './backend-origins';
import {
  ROUTER_ADDRESS, QUOTER_ADDRESS, FEES_ROUTER_ADDRESS, OPTICAL_REGISTRY_ADDRESS,
  USDL_ADDRESS, PROTOCOL_CONFIG_ADDRESS, LAUNCHPAD_ADDRESS,
} from './network/contracts';
import { CORE_DEPLOYMENT } from './network/deployment';
import type { AgentConnection } from '@/widgets/agents/agent-connection';

export function launchpadAgentInterfaceDocument() {
  return {
    ...agentInterfaceDescription(),
    endpoints: { walletGateway: walletGatewayApiBase(), data: backendOrigins().data },
    chain: { id: CORE_DEPLOYMENT.chainId, name: 'Paxeer', nativeCurrency: { symbol: CORE_DEPLOYMENT.nativeSymbol, decimals: CORE_DEPLOYMENT.nativeDecimals } },
    contracts: {
      router: ROUTER_ADDRESS, quoter: QUOTER_ADDRESS, feesRouter: FEES_ROUTER_ADDRESS,
      opticalRegistry: OPTICAL_REGISTRY_ADDRESS, usdl: USDL_ADDRESS,
      protocolConfig: PROTOCOL_CONFIG_ADDRESS, launchpad: LAUNCHPAD_ADDRESS,
    },
  };
}

export function agentConnectionConfiguration(): AgentConnection {
  try {
    const document = launchpadAgentInterfaceDocument();
    return { status: 'ready', entry: document.entry, environment: document.environment, endpoints: document.endpoints };
  } catch (cause) {
    if (!(cause instanceof BackendOriginError)) throw cause;
    return { status: 'unavailable' };
  }
}

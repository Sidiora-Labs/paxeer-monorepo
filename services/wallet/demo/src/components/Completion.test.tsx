import { existsSync, readFileSync, statSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from 'vitest';

type RetiredSurface = {
  id: string;
  path: string;
  pattern?: RegExp;
};

const retiredSurfaces: readonly RetiredSurface[] = [
  {
    id: 'interface.12.err-sdk-unauthorized-custody-not-supported',
    path: 'sdk/wallet/src/errors.ts',
    pattern: /['"]custody_not_supported['"]/m,
  },
  {
    id: 'interface.12.fmt-att-abi-layerxanchor',
    path: 'services/wallet/attestor/internal/policy/evm/abi/layerxanchor.json',
  },
  {
    id: 'interface.12.fmt-att-abi-layerxcustody',
    path: 'services/wallet/attestor/internal/policy/evm/abi/layerxcustody.json',
  },
  {
    id: 'interface.12.go-att-evm-custody-address',
    path: 'services/wallet/attestor/internal/policy/evm/digest.go',
    pattern: /\bCustodyAddress\b/m,
  },
  {
    id: 'interface.12.go-att-evm-custody-consent',
    path: 'services/wallet/attestor/internal/policy/evm/digest.go',
    pattern: /\bCustodyConsent\b/m,
  },
  {
    id: 'interface.12.go-att-evm-custody-consent-matches',
    path: 'services/wallet/attestor/internal/policy/evm/digest.go',
    pattern: /\bMatches\b/m,
  },
  {
    id: 'interface.12.go-att-evm-custody-domain',
    path: 'services/wallet/attestor/internal/policy/evm/digest.go',
    pattern: /\bCustodyDomain\b/m,
  },
  {
    id: 'interface.12.go-att-evm-decode-custody',
    path: 'services/wallet/attestor/internal/policy/evm/digest.go',
    pattern: /\bDecodeCustody\b/m,
  },
  {
    id: 'interface.12.go-att-lx-activity-request',
    path: 'services/wallet/attestor/internal/policy/lx/lx.go',
    pattern: /\bActivityRequest\b/m,
  },
  {
    id: 'interface.12.go-att-lx-evaluator-evaluate-activity',
    path: 'services/wallet/attestor/internal/policy/lx/lx.go',
    pattern: /\bEvaluateActivity\b/m,
  },
  {
    id: 'interface.12.go-att-server-custody-proof-json',
    path: 'services/wallet/attestor/internal/server/sign.go',
    pattern: /\bCustodyProofJSON\b/m,
  },
  {
    id: 'interface.12.go-att-server-kind-custody',
    path: 'services/wallet/attestor/internal/server/sign.go',
    pattern: /\bKindCustody\b/m,
  },
  {
    id: 'interface.12.http-app-page-account-deposit',
    path: 'apps/wallet/src/app/account/deposit/page.tsx',
  },
  {
    id: 'interface.12.key-app-paxeer-custody-v2',
    path: 'apps/wallet/src/account/DepositFlow.tsx',
    pattern: /paxeer:custody:v2:/m,
  },
  {
    id: 'interface.12.key-att-ledger-session-custody',
    path: 'services/wallet/attestor/internal/server/sign.go',
    pattern: /['"]custody-['"]/m,
  },
  {
    id: 'interface.12.key-gw-table-layerx-accounts',
    path: 'services/wallet/gateway/migrations/005_agent_actions.sql',
    pattern: /\bcreate\s+table\s+(?:if\s+not\s+exists\s+)?layerx_accounts\b/mi,
  },
  {
    id: 'interface.12.key-gw-table-wallet-custody-authorizations',
    path: 'services/wallet/gateway/migrations/013_wallet_custody_authorizations.sql',
    pattern: /\bcreate\s+table\s+(?:if\s+not\s+exists\s+)?wallet_custody_authorizations\b/mi,
  },
  {
    id: 'interface.12.key-gw-table-wallet-custody-submissions',
    path: 'services/wallet/gateway/migrations/011_wallet_custody_submissions.sql',
    pattern: /\bcreate\s+table\s+(?:if\s+not\s+exists\s+)?wallet_custody_submissions\b/mi,
  },
  {
    id: 'interface.12.rpc-sdk-provider-paxeer-custody-status',
    path: 'sdk/wallet/src/provider.ts',
    pattern: /\bcase\s+['"]paxeer_custodyStatus['"]\s*:|\b(?:paxeer_custodyStatus|custodyStatus)\s*\(/m,
  },
  {
    id: 'interface.12.rpc-sdk-provider-paxeer-prepare-custody',
    path: 'sdk/wallet/src/provider.ts',
    pattern: /\bcase\s+['"]paxeer_prepareCustody['"]\s*:|\b(?:paxeer_prepareCustody|prepareCustody)\s*\(/m,
  },
  {
    id: 'interface.12.rpc-sdk-provider-paxeer-recover-custody',
    path: 'sdk/wallet/src/provider.ts',
    pattern: /\bcase\s+['"]paxeer_recoverCustody['"]\s*:|\b(?:paxeer_recoverCustody|recoverCustody)\s*\(/m,
  },
  {
    id: 'interface.12.rpc-sdk-provider-paxeer-restore-custody',
    path: 'sdk/wallet/src/provider.ts',
    pattern: /\bcase\s+['"]paxeer_restoreCustody['"]\s*:|\b(?:paxeer_restoreCustody|restoreCustody)\s*\(/m,
  },
  {
    id: 'interface.12.rpc-sdk-provider-paxeer-sign-custody',
    path: 'sdk/wallet/src/provider.ts',
    pattern: /\bcase\s+['"]paxeer_signCustody['"]\s*:|\b(?:paxeer_signCustody|signCustody)\s*\(/m,
  },
  {
    id: 'interface.12.ts-app-account-deposit-page',
    path: 'apps/wallet/src/app/account/deposit/page.tsx',
    pattern: /\bAccountDepositPage\b/m,
  },
  {
    id: 'interface.12.ts-app-beneficiary-of',
    path: 'apps/wallet/src/account/custody.ts',
    pattern: /\bbeneficiaryOf\b/m,
  },
  {
    id: 'interface.12.ts-app-custody-abi',
    path: 'apps/wallet/src/account/custody.ts',
    pattern: /\bCUSTODY_ABI\b/m,
  },
  {
    id: 'interface.12.ts-app-custody-call',
    path: 'apps/wallet/src/account/custody.ts',
    pattern: /\bCustodyCall\b/m,
  },
  {
    id: 'interface.12.ts-app-custody-deposit-error',
    path: 'apps/wallet/src/account/custody.ts',
    pattern: /\bCustodyDepositError\b/m,
  },
  {
    id: 'interface.12.ts-app-custody-handoff',
    path: 'apps/wallet/src/account/custody.ts',
    pattern: /\bcustodyHandoff\b/m,
  },
  {
    id: 'interface.12.ts-app-custody-handoff-domain',
    path: 'apps/wallet/src/account/custody.ts',
    pattern: /\bCUSTODY_HANDOFF_DOMAIN\b/m,
  },
  {
    id: 'interface.12.ts-app-custody-precompile',
    path: 'apps/wallet/src/account/custody.ts',
    pattern: /\bCUSTODY_PRECOMPILE\b/m,
  },
  {
    id: 'interface.12.ts-app-deposit-flow',
    path: 'apps/wallet/src/account/DepositFlow.tsx',
    pattern: /\bDepositFlow\b/m,
  },
  {
    id: 'interface.12.ts-app-deposit-token-calldata',
    path: 'apps/wallet/src/account/custody.ts',
    pattern: /\bdepositTokenCalldata\b/m,
  },
  {
    id: 'interface.12.ts-gw-custody-abi',
    path: 'services/wallet/gateway/src/attestor/client.ts',
    pattern: /\bCUSTODY_ABI\b/m,
  },
  {
    id: 'interface.12.ts-gw-custody-domain',
    path: 'services/wallet/gateway/src/attestor/client.ts',
    pattern: /\bCUSTODY_DOMAIN\b/m,
  },
  {
    id: 'interface.12.ts-gw-custody-target',
    path: 'services/wallet/gateway/src/attestor/client.ts',
    pattern: /\bCUSTODY_TARGET\b/m,
  },
  {
    id: 'interface.12.ts-gw-decode-custody',
    path: 'services/wallet/gateway/src/attestor/client.ts',
    pattern: /\bdecodeCustody\b/m,
  },
  {
    id: 'interface.12.ts-sdk-custody-authorization',
    path: 'sdk/wallet/src/provider.ts',
    pattern: /\bCustodyAuthorization\b/m,
  },
  {
    id: 'interface.12.ts-sdk-custody-submission-status',
    path: 'sdk/wallet/src/provider.ts',
    pattern: /\bCustodySubmissionStatus\b/m,
  },
  {
    id: 'interface.12.ts-sdk-custody-target',
    path: 'sdk/wallet/src/provider.ts',
    pattern: /\bCUSTODY_TARGET\b/m,
  },
  {
    id: 'interface.12.ts-sdk-decode-custody-authorization',
    path: 'sdk/wallet/src/provider.ts',
    pattern: /\bdecodeCustodyAuthorization\b/m,
  },
  {
    id: 'interface.12.ts-sdk-decode-custody-status',
    path: 'sdk/wallet/src/provider.ts',
    pattern: /\bdecodeCustodyStatus\b/m,
  },
  {
    id: 'interface.12.ts-sdk-decode-kernel-status',
    path: 'sdk/wallet/src/endpoint.ts',
    pattern: /\bdecodeKernelStatus\b/m,
  },
  {
    id: 'interface.12.ts-sdk-encode-custody-authorization',
    path: 'sdk/wallet/src/provider.ts',
    pattern: /\bencodeCustodyAuthorization\b/m,
  },
  {
    id: 'interface.12.ts-sdk-kernel-status',
    path: 'sdk/wallet/src/types.ts',
    pattern: /\bKernelStatus\b/m,
  },
  {
    id: 'interface.12.ts-sdk-sign-custody-response',
    path: 'sdk/wallet/src/types.ts',
    pattern: /\bSignCustodyResponse\b/m,
  },
  {
    id: 'interface.12.ts-sdk-wallet-interface-sign-custody',
    path: 'sdk/wallet/src/wallet.ts',
    pattern: /\bsignCustody\b/m,
  },
];

const plannedPaths = [
  'apps/wallet/src/account/custody-retired.test.ts',
  'apps/wallet/src/account/test/endpoint.test.ts',
  'apps/wallet/src/app/account/page.test.tsx',
  'apps/wallet/src/surfaces/useSurface.test.ts',
  'sdk/wallet/test/custody-retired.test.ts',
  'sdk/wallet/test/errors.test.ts',
  'sdk/wallet/test/lx-retired.test.ts',
  'sdk/wallet/test/merged-types.test.ts',
  'services/wallet/attestor/internal/server/custody_retired_test.go',
  'services/wallet/gateway/migrations/019_drop_retired_layerx_custody.sql',
  'services/wallet/gateway/test/agent-precompiles-eip712.test.ts',
  'services/wallet/gateway/test/agent-precompiles-list.test.ts',
  'services/wallet/gateway/test/agent-precompiles-scheduler.test.ts',
  'services/wallet/gateway/test/agent-precompiles-streams.test.ts',
  'services/wallet/gateway/test/agent-precompiles-tee.test.ts',
  'services/wallet/gateway/test/attestor-kinds.test.ts',
  'services/wallet/gateway/test/migrations-retired.test.ts',
];

function walletCompletionRetiredSurfacesAbsent() {
  const root = resolve(__dirname, '../../../../..');
  const remainingSurfaces: string[] = [];
  const missingPaths: string[] = [];

  expect(retiredSurfaces).toHaveLength(47);
  expect(new Set(retiredSurfaces.map((surface) => surface.id)).size).toBe(47);
  expect(plannedPaths).toHaveLength(17);
  expect(new Set(plannedPaths).size).toBe(17);

  for (const surface of retiredSurfaces) {
    const absolutePath = resolve(root, surface.path);
    if (!existsSync(absolutePath)) continue;
    if (!surface.pattern || surface.pattern.test(readFileSync(absolutePath, 'utf8'))) {
      remainingSurfaces.push(`${surface.id}: ${surface.path}`);
    }
  }

  for (const path of plannedPaths) {
    const absolutePath = resolve(root, path);
    if (!existsSync(absolutePath) || !statSync(absolutePath).isFile()) {
      missingPaths.push(path);
    }
  }

  expect(
    { remainingSurfaces, missingPaths },
    'domain 12 completion requires every retirement and planned file',
  ).toEqual({
    remainingSurfaces: [],
    missingPaths: [],
  });
}

test(walletCompletionRetiredSurfacesAbsent);

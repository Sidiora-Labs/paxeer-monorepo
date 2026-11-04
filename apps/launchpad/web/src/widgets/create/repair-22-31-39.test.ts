import { expect, test } from 'vitest';
import { type Address, type Hex } from 'viem';
import { creationFunding } from './creation-adapters';
import {
  funding,
  verifyCost,
  verifyCreationApproval,
  type ApprovalContext,
} from './creation-domain';
import {
  a,
  h,
  owner,
  native,
  pool,
  deployment,
  creationWireDraft as draft,
  creationWireFixture as fixture,
  abi,
  revision,
  trade,
} from './creation-wire.fixture';

export const repair223139Record = {
  findings: {
    'native-client-spender': 'nativeClientSpenderCreationAndFirstBuy',
    'native-domain-spender': 'nativeDomainSpenderProofRejectsHostileContexts',
    'approval-admission-cycle': 'approvalAdmissionZeroAllowanceController',
  },
  baseline:
    'No unchanged-tree verify was run: the bounded implementation-first gate order prohibits a preliminary verify.',
  wiring: [
    'modules/launchpad/keeper/custody.go — native launchpad domain: qualify zero-allowance creation and first purchase on paxd with this native-spender client; the Node gate exercises the provider/codec/controller boundary using an in-memory ledger peer, not paxd.',
    'apps/launchpad/web/src/core/runtime-authority.ts — web runtime authority domain: transport creationStage in RuntimePreflightRequest when consolidating the scoped, request-hashed creation transport.',
    'sdk/launchpad/shared/config/capabilities/preflight.go — shared configuration domain: include the new approval stage in the owning Go qualification; the declared Node gate does not compile or execute Go.',
  ],
} as const;

async function nativeClientSpenderCreationAndFirstBuy() {
  const f = await fixture();
  try {
    const d = draft();
    const read = await f.adapter.readFunding(d, f.scope);
    expect(read.gasStage).toBe('approval');
    expect(read.cost.approvalSpender).toBe(native);
    expect(creationFunding(read).state).toBe('approval_required');
    const approval = await f.adapter.approve('1000000', f.scope);
    await f.adapter.approvalProof(approval, '1000000', f.scope);
    expect(f.peer.ledger.allowance(owner, native)).toBe(1_000_000n);
    expect(f.peer.ledger.allowance(owner, deployment.router)).toBe(0n);
    const create = await f.adapter.sendCreation(d, f.scope);
    const resolved = await f.adapter.resolve(create, owner, undefined, d);
    expect(resolved.state).toBe('resolved');
    expect(f.peer.ledger.credits).toBe(1_000_000n);
    const buyApproval = await f.adapter.approve(d.firstBuyRaw, f.scope);
    await f.adapter.approvalProof(buyApproval, d.firstBuyRaw, f.scope);
    expect(await f.adapter.purchaseAllowance(owner)).toBe(d.firstBuyRaw);
    const quote = await f.adapter.quotePurchase(
      pool,
      d.firstBuyRaw,
      100,
      f.scope,
    );
    const buy = await f.adapter.sendPurchase(quote, f.scope);
    expect((await f.adapter.purchaseReceipt(buy, quote)).status).toBe(
      'success',
    );
    expect(f.peer.ledger.balances.get(owner)).toBe(17_000_000n);
    expect(f.peer.ledger.credits).toBe(3_000_000n);
    expect(f.peer.ledger.allowance(owner, native)).toBe(0n);
    expect(f.peer.allowanceReads.length).toBeGreaterThan(4);
    expect(new Set(f.peer.allowanceReads)).toEqual(new Set([native]));
    for (const tx of [create, buy])
      expect((await f.client.getTransaction({ hash: tx as Hex })).to).toBe(
        deployment.router,
      );
  } finally {
    await f.close();
  }
}

async function nativeDomainSpenderProofRejectsHostileContexts() {
  const f = await fixture();
  try {
    const tx = await f.adapter.approve('1000000', f.scope),
      proof = await f.adapter.approvalProof(tx, '1000000', f.scope);
    const cost = verifyCost(deployment, {
      revision: deployment.revision,
      decimals: 6,
      feeRaw: '1000000',
      gasRaw: '100000',
    });
    expect(
      funding(cost, deployment, {
        usdl: '2000000',
        pax: '1000000',
        allowance: proof.allowanceRaw,
      }).approvalSpender,
    ).toBe(native);
    const expected: ApprovalContext = {
      chainId: 125,
      token: deployment.usdl,
      spender: cost.approvalSpender,
      account: owner,
      manifestRevision: deployment.revision,
      accountEpoch: f.scope.epoch,
      targetRaw: '1000000',
      transactionHash: tx,
      blockHeight: String(proof.receipt.blockNumber),
      blockHash: proof.receipt.blockHash,
    };
    const receipt = { ...expected, status: proof.receipt.status },
      allowance = {
        ...expected,
        allowanceRaw: proof.allowanceRaw,
        observedAt: Date.now(),
      };
    expect(
      verifyCreationApproval(
        expected,
        receipt,
        allowance,
        deployment,
        Date.now(),
        f.scope.epoch,
      ),
    ).toBe(true);
    for (const patch of [
      { spender: deployment.router },
      { spender: a(99) },
      { account: a(99) },
      { manifestRevision: 'changed' },
      { accountEpoch: 2 },
      { chainId: 1 },
      { targetRaw: '2000000' },
      { transactionHash: h(99) },
      { blockHash: h(99) },
      { blockHeight: '99' },
    ])
      expect(() =>
        verifyCreationApproval(
          expected,
          { ...receipt, ...patch },
          allowance,
          deployment,
          Date.now(),
          f.scope.epoch,
        ),
      ).toThrow();
    for (const spender of [deployment.router, a(99)])
      expect(() =>
        verifyCreationApproval(
          { ...expected, spender },
          { ...receipt, spender },
          { ...allowance, spender },
          deployment,
          Date.now(),
          f.scope.epoch,
        ),
      ).toThrow('approval_context_invalid');
    expect(() =>
      verifyCreationApproval(
        expected,
        receipt,
        { ...allowance, allowanceRaw: '1000001' },
        deployment,
        Date.now(),
        f.scope.epoch,
      ),
    ).toThrow('approval_not_verified');
    expect(() =>
      verifyCreationApproval(
        expected,
        receipt,
        { ...allowance, observedAt: Date.now() - 10001 },
        deployment,
        Date.now(),
        f.scope.epoch,
      ),
    ).toThrow('stale_approval_proof');
  } finally {
    await f.close();
  }
}

async function approvalAdmissionZeroAllowanceController() {
  const f = await fixture();
  try {
    const d = draft();
    await expect(f.adapter.admitCreation(d, f.scope)).rejects.toThrow(
      'Creation is unavailable',
    );
    expect(f.peer.transactions.size).toBe(0);
    const reviewed = await f.controller.reviewDraft(d);
    expect(reviewed.read.gasStage).toBe('approval');
    await f.controller.approve();
    expect(f.controller.state.reviewed).toBeUndefined();
    expect(f.controller.state.phase).toBe('draft');
    expect(f.peer.transactions.size).toBe(1);
    expect(
      f.peer.stages
        .filter((value) => value.stage === 'approval')
        .every((value) => value.allowance === 0n),
    ).toBe(true);
    await expect(f.controller.create()).rejects.toThrow('Review this launch');
    expect(f.peer.transactions.size).toBe(1);
    const fresh = await f.controller.reviewDraft(d);
    expect(fresh.read.gasStage).toBe('creation');
    const attempt = await f.controller.create();
    expect(attempt.state).toBe('submitted');
    expect(f.peer.transactions.size).toBe(2);
    expect(f.peer.ledger.credits).toBe(1_000_000n);
    const creations = f.peer.stages.filter(
      (value) => value.stage === 'creation',
    );
    expect(creations.length).toBeGreaterThan(1);
    expect(
      creations
        .slice(1)
        .every((value) => value.allowance === 1_000_000n && value.height > 1n),
    ).toBe(true);
    expect((await f.storage.history())[0].createTxHash).toBe(
      attempt.createTxHash,
    );
  } finally {
    await f.close();
  }
}

async function approvalAdmissionRetainsAuthorityChecks() {
  const f = await fixture();
  try {
    const d = draft();
    for (const change of [
      'expired',
      'capability',
      'stale',
      'epoch',
      'revision',
      'spender',
    ] as const) {
      await f.controller.reviewDraft(d);
      const oldExpiry = f.authority.expiresAt;
      if (change === 'expired') f.authority.expiresAt = Date.now() - 1;
      if (change === 'capability') f.peer.allowed = false;
      if (change === 'stale') f.peer.stale = true;
      if (change === 'epoch') f.scope.epoch++;
      if (change === 'revision')
        f.authority.revision = { ...revision, revisionSha256: '6'.repeat(64) };
      if (change === 'spender')
        f.authority.network = {
          ...f.authority.network,
          network: {
            ...f.authority.network.network,
            addresses: {
              ...f.authority.network.network.addresses,
              LaunchpadPrecompile: a(99),
            },
          },
        };
      await expect(f.controller.approve()).rejects.toThrow();
      expect(f.peer.transactions.size).toBe(0);
      f.authority.expiresAt = oldExpiry;
      f.peer.allowed = true;
      f.peer.stale = false;
      f.authority.revision = revision;
      f.authority.network = {
        ...f.authority.network,
        network: {
          ...f.authority.network.network,
          addresses: {
            ...f.authority.network.network.addresses,
            LaunchpadPrecompile: native,
          },
        },
      };
    }
  } finally {
    await f.close();
  }
}

async function canonicalApprovalProofRejectsWrongAmountWalletAndSpender() {
  const f = await fixture();
  try {
    for (const args of [
      [trade.router, 1_000_000n],
      [native, 2_000_000n],
    ] as const) {
      const tx = await f.wallet.writeContract({
        address: deployment.usdl as Address,
        abi,
        functionName: 'approve',
        args,
      });
      await expect(
        f.adapter.approvalProof(tx, '1000000', f.scope),
      ).rejects.toThrow('reviewed amount and spender');
    }
    const tx = await f.adapter.approve('1000000', f.scope);
    await expect(
      f.adapter.approvalProof(tx, '1000000', { ...f.scope, creator: a(99) }),
    ).rejects.toThrow('Wallet changed');
    f.scope.epoch++;
    await expect(
      f.adapter.approvalProof(tx, '1000000', { ...f.scope, epoch: 1 }),
    ).rejects.toThrow('Wallet changed');
    expect(f.peer.ledger.credits).toBe(0n);
  } finally {
    await f.close();
  }
}

async function creationRequiresFreshFundedAdmissionAfterApproval() {
  const f = await fixture();
  try {
    const d = draft();
    await f.controller.reviewDraft(d);
    await f.controller.approve();
    await f.controller.reviewDraft(d);
    f.peer.allowed = false;
    await expect(f.controller.create()).rejects.toThrow('disabled');
    expect(f.peer.transactions.size).toBe(1);
    expect((await f.storage.history()).length).toBe(0);
    f.peer.allowed = true;
    await f.controller.reviewDraft(d);
    await f.wallet.writeContract({
      address: deployment.usdl as Address,
      abi,
      functionName: 'transfer',
      args: [a(99), 19_000_001n],
    });
    await expect(f.controller.create()).rejects.toThrow('insufficient_usdl');
    expect(f.peer.ledger.credits).toBe(0n);
  } finally {
    await f.close();
  }
}

test(
  nativeClientSpenderCreationAndFirstBuy,
  { timeout: 30000 },
  nativeClientSpenderCreationAndFirstBuy,
);
test(
  nativeDomainSpenderProofRejectsHostileContexts,
  { timeout: 30000 },
  nativeDomainSpenderProofRejectsHostileContexts,
);
test(
  approvalAdmissionZeroAllowanceController,
  { timeout: 30000 },
  approvalAdmissionZeroAllowanceController,
);
test(
  approvalAdmissionRetainsAuthorityChecks,
  { timeout: 30000 },
  approvalAdmissionRetainsAuthorityChecks,
);
test(
  canonicalApprovalProofRejectsWrongAmountWalletAndSpender,
  { timeout: 30000 },
  canonicalApprovalProofRejectsWrongAmountWalletAndSpender,
);
test(
  creationRequiresFreshFundedAdmissionAfterApproval,
  { timeout: 30000 },
  creationRequiresFreshFundedAdmissionAfterApproval,
);

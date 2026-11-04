import { ethers } from 'ethers';
import { getActiveRpcUrl } from '@/lib/constants';
import {
  controllerAbi,
  resolverAbi,
  readPnsContracts,
  PNS_CHAIN_ID,
  PNS_TLD,
  isValidLabel,
  type PriceResult,
} from '@/lib/pns';
import { defineStorageRepository, type StorageRepository } from '@/platform/storage/registry';

const recoveryAbi = [
  ...controllerAbi,
  'function commitments(bytes32) view returns (uint256)',
  'function maxCommitmentAge() view returns (uint256)',
];

export interface PnsDeployment {
  controller: string;
  publicResolver: string;
}

export class PnsController {
  readonly contract: ethers.Contract;

  constructor(
    readonly provider: ethers.Provider,
    readonly deployment: PnsDeployment
  ) {
    this.contract = new ethers.Contract(deployment.controller, recoveryAbi, provider);
  }

  async search(label: string, duration: number) {
    const [available, price] = await Promise.all([
      this.contract.available(label) as Promise<boolean>,
      this.price(label, duration),
    ]);
    return { available, price };
  }

  async price(label: string, duration: number): Promise<PriceResult> {
    const [base, premium] = await this.contract.rentPrice(label, BigInt(duration));
    return { base, premium, total: base + premium };
  }
}

export function configuredPnsController(): PnsController {
  return new PnsController(new ethers.JsonRpcProvider(getActiveRpcUrl()), readPnsContracts());
}

type RegistrationStatus =
  | 'committing'
  | 'waiting'
  | 'ready'
  | 'registering'
  | 'done'
  | 'expired'
  | 'reverted';

interface JournalTransaction {
  kind: 'commit' | 'reveal';
  nonce: number;
  startBlock: number;
  scannedThrough: number;
  data: string;
  value: string;
  hash: string | null;
}

export interface PnsRegistrationIntent {
  account: string;
  chainId: string;
  controller: string;
  resolver: string;
  label: string;
  duration: number;
  secret: string;
  data: string[];
  commitmentHash: string;
  status: RegistrationStatus;
  countdown: number;
  transactions: JournalTransaction[];
}

const object = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);
const uint = (value: unknown): value is number =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
const hash = (value: unknown): value is string =>
  typeof value === 'string' && ethers.isHexString(value, 32);
const address = (value: unknown): value is string =>
  typeof value === 'string' && ethers.isAddress(value);

function parseIntents(value: unknown): PnsRegistrationIntent[] {
  if (!Array.isArray(value) || value.length > 100) throw new Error('Invalid PNS journal');
  for (const entry of value) {
    if (
      !object(entry) ||
      !address(entry.account) ||
      !address(entry.controller) ||
      !address(entry.resolver) ||
      typeof entry.chainId !== 'string' ||
      !/^[1-9]\d*$/.test(entry.chainId) ||
      typeof entry.label !== 'string' ||
      !isValidLabel(entry.label) ||
      !uint(entry.duration) ||
      entry.duration === 0 ||
      !hash(entry.secret) ||
      !hash(entry.commitmentHash) ||
      !uint(entry.countdown) ||
      !Array.isArray(entry.data) ||
      !entry.data.every((data: unknown) => typeof data === 'string' && ethers.isHexString(data)) ||
      !['committing', 'waiting', 'ready', 'registering', 'done', 'expired', 'reverted'].includes(
        String(entry.status)
      ) ||
      !Array.isArray(entry.transactions) ||
      entry.transactions.length > 2
    )
      throw new Error('Invalid PNS registration intent');
    for (const tx of entry.transactions) {
      if (
        !object(tx) ||
        !['commit', 'reveal'].includes(String(tx.kind)) ||
        !uint(tx.nonce) ||
        !uint(tx.startBlock) ||
        !uint(tx.scannedThrough) ||
        typeof tx.data !== 'string' ||
        !ethers.isHexString(tx.data) ||
        typeof tx.value !== 'string' ||
        !/^\d+$/.test(tx.value) ||
        (tx.hash !== null && !hash(tx.hash))
      )
        throw new Error('Invalid PNS transaction journal');
    }
  }
  return value as PnsRegistrationIntent[];
}

const repositories = new Map<string, StorageRepository<PnsRegistrationIntent[]>>();

function registrationRepository(key: string, account: string, chainId: string, controller: string) {
  const existing = repositories.get(key);
  if (existing) return existing;
  const repository = defineStorageRepository<PnsRegistrationIntent[]>({
    id: key,
    key,
    owner: 'pns',
    schema: 'PnsRegistrationIntent[0..1]',
    version: 1,
    area: 'local',
    sensitivity: 'security-relevant',
    retention: 'Until confirmed registration or explicit acknowledgement of expiry/revert',
    quotaBytes: 262_144,
    migration: 'No legacy registration journal exists',
    resetOn: ['reset', 'uninstall'],
    corruption: 'fail-closed',
    prohibitedData: ['mnemonic', 'private key', 'unlock secret', 'session capability'],
    fallback: () => [],
    parse: (value) => {
      const entries = parseIntents(value);
      if (
        entries.length > 1 ||
        entries.some(
          (intent) =>
            intent.account !== account ||
            intent.chainId !== chainId ||
            intent.controller !== controller
        )
      )
        throw new Error('PNS journal identity mismatch');
      return entries;
    },
  });
  repositories.set(key, repository);
  return repository;
}

export function pnsRegistrationTuple(intent: PnsRegistrationIntent) {
  return [
    intent.label,
    intent.account,
    BigInt(intent.duration),
    intent.secret,
    intent.resolver,
    intent.data,
    1,
    ethers.ZeroHash,
  ];
}

export class PnsRegistrationJournal {
  readonly account: string;
  readonly chainId: string;
  readonly controller: string;
  readonly key: string;
  private readonly repository: StorageRepository<PnsRegistrationIntent[]>;

  constructor(account: string, chainId: number | bigint, controller: string) {
    this.account = ethers.getAddress(account).toLowerCase();
    this.chainId = BigInt(chainId).toString();
    this.controller = ethers.getAddress(controller).toLowerCase();
    this.key = `paxport:v1:pns-registration:${this.chainId}:${this.account}:${this.controller}`;
    this.repository = registrationRepository(this.key, this.account, this.chainId, this.controller);
  }

  private matches(intent: PnsRegistrationIntent): boolean {
    return (
      intent.account === this.account &&
      intent.chainId === this.chainId &&
      intent.controller === this.controller
    );
  }

  current(): PnsRegistrationIntent | null {
    return this.repository.read().find((entry) => this.matches(entry)) ?? null;
  }

  save(intent: PnsRegistrationIntent): void {
    if (!this.matches(intent)) throw new Error('PNS journal identity mismatch');
    const entries = this.repository.read().filter((entry) => !this.matches(entry));
    entries.push(intent);
    this.repository.write(entries);
    if (JSON.stringify(this.current()) !== JSON.stringify(intent)) {
      throw new Error('PNS registration could not be saved. No further transaction will be sent.');
    }
  }

  acknowledge(): void {
    const current = this.current();
    if (current && !['done', 'expired', 'reverted'].includes(current.status)) {
      throw new Error(
        'Reconcile the original PNS commitment before starting another registration.'
      );
    }
    this.repository.write(this.repository.read().filter((entry) => !this.matches(entry)));
    if (this.current()) throw new Error('PNS journal could not be cleared');
  }
}

export class PnsRegistrationSession {
  private closed = false;

  constructor(
    readonly journal: PnsRegistrationJournal,
    readonly client: PnsController,
    private readonly changed: (intent: PnsRegistrationIntent) => void,
    private readonly isCurrent: () => boolean
  ) {}

  close(): void {
    this.closed = true;
  }

  private check(): void {
    if (this.closed || !this.isCurrent())
      throw new Error('PNS session changed; resume the saved registration.');
  }

  private publish(intent: PnsRegistrationIntent): void {
    this.journal.save(intent);
    if (!this.closed && this.isCurrent()) this.changed(structuredClone(intent));
  }

  private async bound(signer: ethers.Signer): Promise<ethers.Provider> {
    this.check();
    if (!signer.provider) throw new Error('PNS registration requires a provider');
    const [account, network, readNetwork] = await Promise.all([
      signer.getAddress(),
      signer.provider.getNetwork(),
      this.client.provider.getNetwork(),
    ]);
    this.check();
    if (
      account.toLowerCase() !== this.journal.account ||
      network.chainId.toString() !== this.journal.chainId ||
      readNetwork.chainId !== network.chainId ||
      network.chainId !== BigInt(PNS_CHAIN_ID) ||
      this.client.deployment.controller.toLowerCase() !== this.journal.controller
    ) {
      throw new Error('Resume PNS registration with the original account, chain and controller.');
    }
    return signer.provider;
  }

  private async locked<T>(action: () => Promise<T>): Promise<T> {
    if (!globalThis.navigator?.locks)
      throw new Error('PNS registration requires browser transaction locking');
    return navigator.locks.request(this.journal.key, { ifAvailable: true }, async (lock) => {
      if (!lock) throw new Error('PNS registration is open in another window');
      this.check();
      return action();
    });
  }

  private matches(tx: ethers.TransactionResponse, saved: JournalTransaction): boolean {
    return (
      tx.from.toLowerCase() === this.journal.account &&
      tx.chainId.toString() === this.journal.chainId &&
      tx.nonce === saved.nonce &&
      tx.to?.toLowerCase() === this.journal.controller &&
      tx.data === saved.data &&
      tx.value.toString() === saved.value
    );
  }

  private async observe(
    provider: ethers.Provider,
    intent: PnsRegistrationIntent,
    saved: JournalTransaction
  ) {
    if (!saved.hash) {
      const latest = await provider.getBlockNumber();
      const tags: (number | string)[] = ['pending'];
      for (
        let n = Math.max(saved.startBlock, saved.scannedThrough + 1);
        n <= Math.min(latest, saved.scannedThrough + 128);
        n++
      )
        tags.push(n);
      for (const tag of tags) {
        this.check();
        const block = await provider.getBlock(tag, true);
        if (!block) throw new Error('PNS transaction reconciliation could not read a block');
        for (const txHash of block.transactions) {
          const tx = await provider.getTransaction(txHash);
          if (!tx || tx.from.toLowerCase() !== intent.account || tx.nonce !== saved.nonce) continue;
          if (!this.matches(tx, saved))
            throw new Error('PNS transaction nonce belongs to a different transaction');
          saved.hash = tx.hash;
          break;
        }
        if (typeof tag === 'number') saved.scannedThrough = tag;
        if (saved.hash) break;
      }
      this.publish(intent);
    }
    if (!saved.hash) return null;
    const [tx, receipt] = await Promise.all([
      provider.getTransaction(saved.hash),
      provider.getTransactionReceipt(saved.hash),
    ]);
    this.check();
    if (tx && !this.matches(tx, saved))
      throw new Error('Recovered PNS transaction identity does not match');
    if (receipt && (!tx || (receipt.status !== 0 && receipt.status !== 1)))
      throw new Error('PNS receipt has no verified transaction identity');
    return receipt;
  }

  private async send(
    signer: ethers.Signer,
    intent: PnsRegistrationIntent,
    kind: JournalTransaction['kind'],
    data: string,
    value = 0n
  ) {
    const provider = await this.bound(signer);
    const [nonce, startBlock] = await Promise.all([
      provider.getTransactionCount(intent.account, 'pending'),
      provider.getBlockNumber(),
    ]);
    const saved: JournalTransaction = {
      kind,
      nonce,
      startBlock,
      scannedThrough: Math.max(0, startBlock - 1),
      data,
      value: value.toString(),
      hash: null,
    };
    this.check();
    intent.transactions.push(saved);
    intent.status = kind === 'commit' ? 'committing' : 'registering';
    this.publish(intent);
    this.check();
    try {
      const tx = await signer.sendTransaction({
        to: intent.controller,
        from: intent.account,
        chainId: BigInt(intent.chainId),
        nonce,
        data,
        value,
        gasLimit: kind === 'commit' ? 200000 : 500000,
      });
      saved.hash = tx.hash;
      this.publish(intent);
      if (!this.matches(tx, saved))
        throw new Error('Signer returned a different PNS transaction identity');
    } catch (error) {
      if (object(error) && hash(error.hash)) {
        saved.hash = error.hash;
        this.publish(intent);
      }
      throw error;
    }
  }

  async start(signer: ethers.Signer, label: string, duration: number): Promise<void> {
    await this.locked(async () => {
      await this.bound(signer);
      if (this.journal.current()) throw new Error('Resume the saved PNS registration first');
      if (!isValidLabel(label) || !uint(duration) || duration === 0)
        throw new Error('Invalid PNS registration');
      const result = await this.client.search(label, duration);
      this.check();
      if (!result.available) throw new Error('The name is no longer available');
      const resolver = this.client.deployment.publicResolver.toLowerCase();
      const data = [
        new ethers.Interface(resolverAbi).encodeFunctionData('setAddr', [
          ethers.namehash(`${label}.${PNS_TLD}`),
          this.journal.account,
        ]),
      ];
      const intent: PnsRegistrationIntent = {
        account: this.journal.account,
        chainId: this.journal.chainId,
        controller: this.journal.controller,
        resolver,
        label,
        duration,
        secret: ethers.hexlify(ethers.randomBytes(32)),
        data,
        commitmentHash: ethers.ZeroHash,
        status: 'committing',
        countdown: 0,
        transactions: [],
      };
      intent.commitmentHash = await this.client.contract.makeCommitment(
        pnsRegistrationTuple(intent)
      );
      this.check();
      this.publish(intent);
      await this.send(
        signer,
        intent,
        'commit',
        this.client.contract.interface.encodeFunctionData('commit', [intent.commitmentHash])
      );
    });
  }

  async resume(signer: ethers.Signer, reveal: boolean): Promise<void> {
    await this.locked(async () => {
      const provider = await this.bound(signer);
      const intent = this.journal.current();
      if (!intent) return;
      const expectedHash = await this.client.contract.makeCommitment(pnsRegistrationTuple(intent));
      this.check();
      if (expectedHash !== intent.commitmentHash)
        throw new Error('Saved PNS commitment does not match its reveal parameters');
      const commit = intent.transactions.find((tx) => tx.kind === 'commit');
      const registration = intent.transactions.find((tx) => tx.kind === 'reveal');
      if (registration) {
        const receipt = await this.observe(provider, intent, registration);
        intent.status = receipt ? (receipt.status === 1 ? 'done' : 'reverted') : 'registering';
        this.publish(intent);
        return;
      }
      if (!commit)
        throw new Error('PNS commitment was saved before broadcast; reconcile it before retrying');
      const receipt = await this.observe(provider, intent, commit);
      if (!receipt) {
        intent.status = 'committing';
        this.publish(intent);
        return;
      }
      if (receipt.status === 0) {
        intent.status = 'reverted';
        this.publish(intent);
        return;
      }
      const [committedAt, minAge, maxAge, block] = await Promise.all([
        this.client.contract.commitments(intent.commitmentHash) as Promise<bigint>,
        this.client.contract.minCommitmentAge() as Promise<bigint>,
        this.client.contract.maxCommitmentAge() as Promise<bigint>,
        provider.getBlock('latest'),
      ]);
      this.check();
      if (!block || committedAt === 0n || minAge < 0n || maxAge <= minAge)
        throw new Error('PNS commitment age could not be reconciled');
      const age = BigInt(block.timestamp) - committedAt;
      intent.countdown = Number(age < minAge ? minAge - age : 0n);
      if (!Number.isSafeInteger(intent.countdown))
        throw new Error('PNS commitment age is outside supported bounds');
      intent.status = age >= maxAge ? 'expired' : age < minAge ? 'waiting' : 'ready';
      this.publish(intent);
      if (intent.status !== 'ready' || !reveal) return;
      const price = await this.client.price(intent.label, intent.duration);
      this.check();
      await this.send(
        signer,
        intent,
        'reveal',
        this.client.contract.interface.encodeFunctionData('register', [
          pnsRegistrationTuple(intent),
        ]),
        (price.total * 110n) / 100n
      );
    });
  }
}

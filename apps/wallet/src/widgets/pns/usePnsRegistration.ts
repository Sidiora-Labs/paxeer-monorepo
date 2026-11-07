'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { ethers } from 'ethers';
import { PNS_CHAIN_ID } from '@/lib/pns';
import type { PnsSearchResult } from './usePnsSearch';
import {
  PnsRegistrationJournal,
  PnsRegistrationSession,
  type PnsController,
  type PnsRegistrationIntent,
} from './pnsRegistration';

export interface PnsRegistrationContext {
  account: string | null;
  chainId: number | null;
  client: PnsController;
  getSigner: () => Promise<ethers.Signer>;
  isCurrent: () => boolean;
}

export function usePnsRegistration(context: PnsRegistrationContext) {
  const latest = useRef(context);
  latest.current = context;
  const { account, chainId, client } = context;
  const scope =
    account && chainId ? `${account.toLowerCase()}:${chainId}:${client.deployment.controller}` : '';
  const [state, setState] = useState<{
    scope: string;
    intent: PnsRegistrationIntent | null;
    error: string;
    busy: boolean;
  }>({ scope: '', intent: null, error: '', busy: false });
  const owner = useRef<{
    session: PnsRegistrationSession;
    journal: PnsRegistrationJournal;
    busy: boolean;
    automatic: boolean;
  } | null>(null);

  useEffect(() => {
    setState({ scope, intent: null, error: '', busy: false });
    if (!account || chainId !== PNS_CHAIN_ID) return;
    const journal = new PnsRegistrationJournal(account, chainId, client.deployment.controller);
    const session = new PnsRegistrationSession(
      journal,
      client,
      (intent) => {
        if (owner.current?.session === session) setState((previous) => ({ ...previous, intent }));
      },
      () =>
        latest.current.account?.toLowerCase() === account.toLowerCase() &&
        latest.current.chainId === chainId &&
        latest.current.client === client &&
        latest.current.isCurrent()
    );
    const active = { session, journal, busy: false, automatic: false };
    owner.current = active;
    const report = (reason: unknown) => {
      if (owner.current === active)
        setState((previous) => ({
          ...previous,
          error: reason instanceof Error ? reason.message : 'PNS recovery failed',
        }));
    };
    try {
      const intent = journal.current();
      setState({ scope, intent, error: '', busy: !!intent });
      if (intent) {
        active.busy = true;
        void context
          .getSigner()
          .then((signer) => session.resume(signer, false))
          .catch(report)
          .finally(() => {
            active.busy = false;
            if (owner.current === active) setState((previous) => ({ ...previous, busy: false }));
          });
      }
    } catch (reason) {
      report(reason);
    }
    const timer = setInterval(() => {
      if (!active.automatic || active.busy) return;
      let intent: PnsRegistrationIntent | null;
      try {
        intent = journal.current();
      } catch (reason) {
        active.automatic = false;
        report(reason);
        return;
      }
      if (!intent || ['done', 'expired', 'reverted'].includes(intent.status)) {
        active.automatic = false;
        return;
      }
      active.busy = true;
      void latest.current
        .getSigner()
        .then((signer) => session.resume(signer, true))
        .catch((reason) => {
          active.automatic = false;
          report(reason);
        })
        .finally(() => {
          active.busy = false;
        });
    }, 1000);
    return () => {
      clearInterval(timer);
      session.close();
      if (owner.current === active) owner.current = null;
    };
  }, [account, chainId, client, scope]);

  const run = useCallback(async (label?: string, duration?: number) => {
    const active = owner.current;
    if (!active || active.busy) return;
    active.busy = true;
    active.automatic = false;
    setState((previous) => ({ ...previous, busy: true, error: '' }));
    try {
      const signer = await latest.current.getSigner();
      if (label !== undefined && duration !== undefined)
        await active.session.start(signer, label, duration);
      await active.session.resume(signer, true);
      active.automatic = true;
    } catch (reason) {
      if (owner.current === active)
        setState((previous) => ({
          ...previous,
          error: reason instanceof Error ? reason.message : 'PNS registration failed',
        }));
    } finally {
      active.busy = false;
      if (owner.current === active) setState((previous) => ({ ...previous, busy: false }));
    }
  }, []);

  const acknowledge = useCallback(() => {
    const active = owner.current;
    if (!active || active.busy) return;
    try {
      active.journal.acknowledge();
      active.automatic = false;
      setState((previous) => ({ ...previous, intent: null, error: '' }));
    } catch (reason) {
      setState((previous) => ({
        ...previous,
        error: reason instanceof Error ? reason.message : 'PNS recovery failed',
      }));
    }
  }, []);
  return {
    intent: state.scope === scope ? state.intent : null,
    error: state.scope === scope ? state.error : '',
    busy: state.scope === scope && state.busy,
    enabled: !!owner.current && state.scope === scope,
    start: (validated: () => PnsSearchResult | null) => {
      const result = validated();
      return result?.available ? run(result.label, result.duration) : Promise.resolve();
    },
    resume: () => run(),
    acknowledge,
  };
}

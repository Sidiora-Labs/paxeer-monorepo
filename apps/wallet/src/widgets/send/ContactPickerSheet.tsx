'use client';

/**
 * Bottom-sheet address picker.
 *
 * Three sections shown in priority order:
 *  1. My Accounts   — other accounts in this wallet
 *  2. Recent        — last 5 successful send recipients
 *  3. Contacts      — saved address-book entries
 *
 * A unified search bar filters across all three sections simultaneously.
 */

import { useMemo, useState } from 'react';
import { SvgIcon } from '@/components/ui/SvgIcon';
import { Dialog } from '@/components/ui/primitives';
import { getAvatarPath } from '@/lib/avatar';
import type { Contact } from '@/lib/contacts';
import type { RecentRecipient } from '@/lib/recentRecipients';
import Image from 'next/image';
import { useLocale } from '@/providers/LocaleProvider';

export interface OwnAccount {
  address: string;
  name: string;
}

export interface ContactPickerSheetProps {
  open: boolean;
  onClose: () => void;
  contacts: Contact[];
  ownAccounts: OwnAccount[]; // other accounts in the wallet (active excluded)
  recentRecipients: RecentRecipient[];
  onSelect: (address: string) => void;
}

function SectionLabel({ label }: { label: string }) {
  return (
    <p className="text-size-11 font-bold text-pax-muted/50 uppercase tracking-widest px-1 pt-3 pb-1 first:pt-0">
      {label}
    </p>
  );
}

function AddressRow({
  avatar,
  initials,
  name,
  address,
  tag,
  onSelect,
  onClose,
}: {
  avatar?: string;
  initials?: string;
  name: string;
  address: string;
  tag?: string;
  onSelect: (a: string) => void;
  onClose: () => void;
}) {
  return (
    <button
      type="button"
      onClick={() => {
        onSelect(address);
        onClose();
      }}
      className="min-h-[var(--touch-target)] w-full flex items-center gap-3 px-3 py-3 rounded-xl bg-pax-control hover:bg-pax-overlay transition-all press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
    >
      {avatar ? (
        <Image
          src={avatar}
          alt={name}
          className="w-9 h-9 rounded-full object-cover shrink-0"
          width={36}
          height={36}
        />
      ) : (
        <div className="w-9 h-9 rounded-full bg-pax-accent/10 flex items-center justify-center shrink-0">
          <span className="text-xs font-bold text-pax-accent">{initials}</span>
        </div>
      )}
      <div className="flex-1 text-left min-w-0">
        <div className="flex items-center gap-2">
          <p className="text-sm font-medium truncate">{name}</p>
          {tag && (
            <span className="shrink-0 text-size-9 font-semibold uppercase tracking-wide px-1.5 py-0.5 rounded-full bg-pax-control text-pax-muted">
              {tag}
            </span>
          )}
        </div>
        <p className="text-size-11 text-pax-muted font-mono truncate">
          {address.slice(0, 8)}…{address.slice(-6)}
        </p>
      </div>
    </button>
  );
}

export function ContactPickerSheet({
  open,
  onClose,
  contacts,
  ownAccounts,
  recentRecipients,
  onSelect,
}: ContactPickerSheetProps) {
  const { p, t } = useLocale();
  const [search, setSearch] = useState('');

  const needle = search.toLowerCase();

  const filteredAccounts = useMemo(
    () =>
      needle
        ? ownAccounts.filter(
            (a) => a.name.toLowerCase().includes(needle) || a.address.toLowerCase().includes(needle)
          )
        : ownAccounts,
    [ownAccounts, needle]
  );

  const filteredRecent = useMemo(() => {
    const list = needle
      ? recentRecipients.filter(
          (r) =>
            (r.label?.toLowerCase().includes(needle) ?? false) ||
            r.address.toLowerCase().includes(needle)
        )
      : recentRecipients;
    // Deduplicate against own accounts so same address doesn't appear twice
    const ownAddrs = new Set(ownAccounts.map((a) => a.address.toLowerCase()));
    return list.filter((r) => !ownAddrs.has(r.address.toLowerCase()));
  }, [recentRecipients, ownAccounts, needle]);

  const filteredContacts = useMemo(() => {
    const list = needle
      ? contacts.filter(
          (c) => c.name.toLowerCase().includes(needle) || c.address.toLowerCase().includes(needle)
        )
      : contacts;
    // Deduplicate against own accounts + recent
    const seen = new Set([
      ...ownAccounts.map((a) => a.address.toLowerCase()),
      ...recentRecipients.map((r) => r.address.toLowerCase()),
    ]);
    return list.filter((c) => !seen.has(c.address.toLowerCase()));
  }, [contacts, ownAccounts, recentRecipients, needle]);

  const totalCount = filteredAccounts.length + filteredRecent.length + filteredContacts.length;
  const showSearch = ownAccounts.length + recentRecipients.length + contacts.length > 3;

  if (!open) return null;

  return (
    <Dialog open={open} onClose={onClose} title={p.sendTo} variant="sheet">
      <div className="flex max-h-[80dvh] flex-col">
        {/* Header */}
        <div className="flex items-center justify-between mb-4 shrink-0">
          <h3 className="text-base font-bold">{p.sendTo}</h3>
          <button
            type="button"
            onClick={onClose}
            aria-label={t.common.close}
            className="min-h-[var(--touch-target)] min-w-[var(--touch-target)] p-2 rounded-full bg-pax-control press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
          >
            <SvgIcon name="x" className="w-4 h-4" />
          </button>
        </div>

        {/* Search */}
        {showSearch && (
          <div className="relative mb-3 shrink-0">
            <SvgIcon
              name="search"
              className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5"
            />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={p.searchAddress}
              className="min-h-[var(--touch-target)] w-full pl-9 pr-4 py-2.5 rounded-xl bg-pax-control text-sm outline-none placeholder:text-pax-muted focus-visible:ring-2 focus-visible:ring-pax-accent"
            />
          </div>
        )}

        {/* List */}
        <div className="overflow-y-auto flex-1 space-y-0.5">
          {totalCount === 0 && (
            <p className="text-xs text-pax-muted text-center py-8">{p.noResults}</p>
          )}

          {/* ── My Accounts ── */}
          {filteredAccounts.length > 0 && (
            <>
              <SectionLabel label={p.myAccounts} />
              {filteredAccounts.map((acc) => (
                <AddressRow
                  key={acc.address}
                  avatar={getAvatarPath(acc.address)}
                  name={acc.name}
                  address={acc.address}
                  tag="wallet"
                  onSelect={onSelect}
                  onClose={onClose}
                />
              ))}
            </>
          )}

          {/* ── Recent ── */}
          {filteredRecent.length > 0 && (
            <>
              <SectionLabel label={p.recent} />
              {filteredRecent.map((r) => (
                <AddressRow
                  key={r.address}
                  initials={r.label ? r.label[0].toUpperCase() : '#'}
                  name={r.label || `${r.address.slice(0, 6)}…${r.address.slice(-4)}`}
                  address={r.address}
                  tag="recent"
                  onSelect={onSelect}
                  onClose={onClose}
                />
              ))}
            </>
          )}

          {/* ── Contacts ── */}
          {filteredContacts.length > 0 && (
            <>
              <SectionLabel label={p.contacts} />
              {filteredContacts.map((c) => (
                <AddressRow
                  key={c.id}
                  initials={c.name[0]?.toUpperCase() || '?'}
                  name={c.name}
                  address={c.address}
                  onSelect={onSelect}
                  onClose={onClose}
                />
              ))}
            </>
          )}
        </div>
      </div>
    </Dialog>
  );
}

/** Bounded session cache of genuine missing profiles; a successful save invalidates the entry. */
const storageKey = 'sidiora.profile-absent.v1';
const valid = /^0x[0-9a-f]{40}$/;
export const profileAbsenceCapacity = 512;
export const profileAbsenceTtlMs = 30 * 60 * 1000;
export const profileAbsenceStorageLimit = 64 * 1024;
let absent: Map<string, number> | undefined;

function storage(): Storage | null {
  try { return typeof window === 'undefined' ? null : window.sessionStorage; } catch { return null; }
}

function persist(entries: Map<string, number>): void {
  try {
    storage()?.setItem(storageKey, JSON.stringify({ version: 2, entries: [...entries] }));
  } catch { /* storage full or blocked: bounded memory still holds it */ }
}

function prune(entries: Map<string, number>, now: number): boolean {
  let changed = false;
  for (const [wallet, savedAt] of entries) {
    if (savedAt > now || now - savedAt >= profileAbsenceTtlMs) {
      entries.delete(wallet);
      changed = true;
    }
  }
  while (entries.size > profileAbsenceCapacity) {
    entries.delete(entries.keys().next().value!);
    changed = true;
  }
  return changed;
}

function known(): Map<string, number> {
  const now = Date.now();
  if (absent) {
    if (prune(absent, now)) persist(absent);
    return absent;
  }
  absent = new Map();
  try {
    const raw = storage()?.getItem(storageKey);
    // Bound the input before JSON parsing; legacy entries have no trustworthy age.
    if (raw && raw.length <= profileAbsenceStorageLimit) {
      const parsed: unknown = JSON.parse(raw);
      if (parsed && typeof parsed === 'object' && 'version' in parsed && parsed.version === 2 && 'entries' in parsed && Array.isArray(parsed.entries) && parsed.entries.length <= profileAbsenceCapacity) {
        for (const entry of parsed.entries) {
          if (Array.isArray(entry) && entry.length === 2 && typeof entry[0] === 'string' && valid.test(entry[0]) && typeof entry[1] === 'number' && Number.isSafeInteger(entry[1]) && entry[1] > 0 && entry[1] <= now && now - entry[1] < profileAbsenceTtlMs) {
            absent.delete(entry[0]);
            absent.set(entry[0], entry[1]);
          }
        }
      }
    }
  } catch { /* unreadable storage: start empty */ }
  persist(absent);
  return absent;
}

export function isProfileAbsent(wallet: string): boolean {
  const entries = known();
  const key = wallet.toLowerCase();
  const savedAt = entries.get(key);
  if (savedAt === undefined) return false;
  // Refresh LRU order without extending the age of the negative answer.
  entries.delete(key);
  entries.set(key, savedAt);
  persist(entries);
  return true;
}

export function rememberProfileAbsent(wallet: string): void {
  const key = wallet.toLowerCase();
  if (!valid.test(key)) return;
  const entries = known();
  const savedAt = entries.get(key) ?? Date.now();
  entries.delete(key);
  entries.set(key, savedAt);
  prune(entries, Date.now());
  persist(entries);
}

/** Called when a wallet publishes a profile, so the next lookup reads it. */
export function forgetProfileAbsence(wallet: string): void {
  const entries = known();
  if (entries.delete(wallet.toLowerCase())) persist(entries);
}

/** Test seam for suites that need a fresh session; not used by the app. */
export function resetProfileAbsenceForSession(): void {
  absent = undefined;
  try { storage()?.removeItem(storageKey); } catch { /* ignore */ }
}

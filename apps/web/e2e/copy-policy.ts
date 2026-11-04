import { BANNED_VOCABULARY, copyEntries } from "../copy/catalog.ts";
import { formatCopy } from "../copy/format.ts";

export function vocabularyPattern(word: string): RegExp {
  const flags = /^[A-Z][A-Z0-9]*$/u.test(word) ? "u" : "iu";
  return new RegExp(`\\b${word.replace(/[.*+?^${}()|[\]\\]/gu, "\\$&")}\\b`, flags);
}

export function forbiddenCopy(message: string): string | undefined {
  return BANNED_VOCABULARY.find((word) => vocabularyPattern(word).test(message));
}

export function onboardingMessages(): readonly string[] {
  return copyEntries.filter((entry) => entry.key.startsWith("onboarding.")).map((entry) =>
    formatCopy(entry.key, { stage: "Activating your account" }));
}

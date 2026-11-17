import { LocaleEnum, type LocaleCode } from "@paxeer-network/i18n";

const locales = {
  en: () => import("../../../../packages/i18n/locales/en.json"),
  ar: () => import("../../../../packages/i18n/locales/ar.json"),
  fa: () => import("../../../../packages/i18n/locales/fa.json"),
  he: () => import("../../../../packages/i18n/locales/he.json"),
  zh: () => import("../../../../packages/i18n/locales/zh.json"),
  ja: () => import("../../../../packages/i18n/locales/ja.json"),
  es: () => import("../../../../packages/i18n/locales/es.json"),
  ko: () => import("../../../../packages/i18n/locales/ko.json"),
  vi: () => import("../../../../packages/i18n/locales/vi.json"),
  de: () => import("../../../../packages/i18n/locales/de.json"),
  fr: () => import("../../../../packages/i18n/locales/fr.json"),
  ru: () => import("../../../../packages/i18n/locales/ru.json"),
  id: () => import("../../../../packages/i18n/locales/id.json"),
  tr: () => import("../../../../packages/i18n/locales/tr.json"),
  it: () => import("../../../../packages/i18n/locales/it.json"),
  pt: () => import("../../../../packages/i18n/locales/pt.json"),
  uk: () => import("../../../../packages/i18n/locales/uk.json"),
  pl: () => import("../../../../packages/i18n/locales/pl.json"),
  nl: () => import("../../../../packages/i18n/locales/nl.json"),
  tc: () => import("../../../../packages/i18n/locales/tc.json"),
} satisfies Record<
  keyof typeof LocaleEnum,
  () => Promise<{ default: Record<string, string> }>
>;

export async function loadNativeLocale(
  language: LocaleCode,
): Promise<Record<string, string>> {
  if (!Object.prototype.hasOwnProperty.call(locales, language)) {
    throw new Error(`Unsupported locale: ${language}`);
  }
  const messages = await locales[language as keyof typeof locales]();
  return messages.default;
}

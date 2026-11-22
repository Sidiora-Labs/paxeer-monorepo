import React, { type CSSProperties } from "react";
import Link from "next/link";
import { ChevronLeftIcon } from "@paxeer-network/ui/icon/chevronLeft";

import { WORKSPACE_THEME_CSS_VARS } from "../theme/workspaceTheme";
import { CookieBanner } from "./CookieBanner";

interface LegalLayoutProps {
  title: string;
  lastUpdated: string;
  children: React.ReactNode;
}

/**
 * Shared layout for Privacy Policy, Terms of Service, and Cookie Policy pages.
 * Renders a centred prose column with a back-to-home link.
 */
export function LegalLayout({
  title,
  lastUpdated,
  children,
}: LegalLayoutProps) {
  return (
    <div
      style={WORKSPACE_THEME_CSS_VARS as CSSProperties}
      className="oui-min-h-full oui-bg-base-10 oui-px-4 oui-py-10"
    >
      <div className="oui-mx-auto oui-max-w-2xl">
        {/* Back link */}
        <Link
          href="/"
          className="oui-inline-flex oui-items-center oui-gap-1.5 oui-text-xs oui-text-base-contrast-54 hover:oui-text-base-contrast oui-transition-colors oui-mb-8"
        >
          <ChevronLeftIcon size={14} />
          Back to HyperPax
        </Link>

        {/* Masthead */}
        <header className="oui-mb-10 oui-pb-6 oui-border-b oui-border-line-12">
          <p className="oui-text-[10px] oui-uppercase oui-tracking-widest oui-font-mono oui-text-base-contrast-54 oui-mb-2">
            Legal
          </p>
          <h1 className="oui-text-2xl oui-font-bold oui-text-base-contrast">
            {title}
          </h1>
          <p className="oui-text-xs oui-text-base-contrast-54 oui-mt-1">
            Last updated: {lastUpdated}
          </p>
        </header>

        {/* Prose */}
        <div className="oui-space-y-6 oui-text-[13px] oui-leading-relaxed oui-text-base-contrast-54 [&_h2]:oui-text-base [&_h2]:oui-font-semibold [&_h2]:oui-text-base-contrast [&_h2]:oui-mt-8 [&_h2]:oui-mb-3 [&_p]:oui-mb-4 [&_ul]:oui-list-disc [&_ul]:oui-pl-5 [&_ul]:oui-space-y-1 [&_a]:oui-text-base-contrast [&_a]:oui-underline [&_a]:oui-underline-offset-2 [&_strong]:oui-text-base-contrast [&_strong]:oui-font-medium">
          {children}
        </div>

        {/* Footer nav */}
        <div className="oui-mt-12 oui-pt-6 oui-border-t oui-border-line-12 oui-flex oui-flex-wrap oui-gap-4 oui-text-[11px] oui-text-base-contrast-54">
          <Link
            href="/privacy"
            className="hover:oui-text-base-contrast oui-transition-colors"
          >
            Privacy Policy
          </Link>
          <Link
            href="/terms"
            className="hover:oui-text-base-contrast oui-transition-colors"
          >
            Terms of Service
          </Link>
          <Link
            href="/cookies"
            className="hover:oui-text-base-contrast oui-transition-colors"
          >
            Cookie Policy
          </Link>
        </div>
      </div>
      <CookieBanner />
    </div>
  );
}

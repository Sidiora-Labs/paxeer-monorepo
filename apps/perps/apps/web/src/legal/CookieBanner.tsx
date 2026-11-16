"use client";

import React from "react";
import Link from "next/link";
import { Button } from "@paxeer-network/ui/button/button";
import { CloseIcon } from "@paxeer-network/ui/icon/close";
import { useCookieConsent } from "./useCookieConsent";

export function CookieBanner() {
  const { value, loading, accept, decline } = useCookieConsent();
  if (loading || value !== null) return null;

  return (
    <div
      role="dialog"
      aria-label="Cookie consent"
      aria-describedby="cookie-consent-description"
      aria-modal="false"
      className="oui-fixed oui-bottom-6 oui-left-1/2 -oui-translate-x-1/2 oui-z-[200] oui-w-full oui-max-w-[560px] oui-px-4"
    >
      <div className="oui-rounded-xl oui-border oui-border-line-12 oui-bg-base-9 oui-shadow-2xl oui-p-5">
        <div className="oui-flex oui-items-start oui-justify-between oui-gap-3 oui-mb-3">
          <span className="oui-text-sm oui-font-semibold oui-text-base-contrast">
            We use cookies
          </span>
          <Button
            type="button"
            onClick={decline}
            aria-label="Dismiss cookie banner"
            variant="text"
            color="secondary"
            size="sm"
          >
            <CloseIcon size={16} />
          </Button>
        </div>
        <p
          id="cookie-consent-description"
          className="oui-text-xs oui-text-base-contrast-54 oui-leading-relaxed oui-mb-4"
        >
          HyperPax uses essential cookies required for the platform to function,
          and optional analytics cookies to help us understand how you use the
          app. We never sell your data or share it with advertisers. Read our{" "}
          <Link
            href="/cookies"
            className="oui-text-base-contrast oui-underline oui-underline-offset-2"
          >
            Cookie Policy
          </Link>{" "}
          and{" "}
          <Link
            href="/privacy"
            className="oui-text-base-contrast oui-underline oui-underline-offset-2"
          >
            Privacy Policy
          </Link>{" "}
          for details.
        </p>
        <div className="oui-flex oui-flex-wrap oui-items-center oui-gap-2 oui-justify-end">
          <Button
            type="button"
            onClick={decline}
            variant="outlined"
            color="secondary"
            size="sm"
          >
            Essential only
          </Button>
          <Button type="button" onClick={accept} size="sm">
            Accept all cookies
          </Button>
        </div>
      </div>
    </div>
  );
}

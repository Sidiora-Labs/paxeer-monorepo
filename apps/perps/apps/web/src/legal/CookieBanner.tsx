"use client";

import React, { useEffect, useRef } from "react";
import Link from "next/link";
import { Button } from "@paxeer-network/ui/button/button";
import { CloseIcon } from "@paxeer-network/ui/icon/close";
import { useCookieConsent } from "./useCookieConsent";

export function CookieBanner() {
  const { value, loading, accept, decline } = useCookieConsent();
  const bannerRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (loading || value !== null) return;
    const revealFocus = (event: FocusEvent) => {
      const target = event.target;
      const banner = bannerRef.current;
      if (
        !(target instanceof HTMLElement) ||
        !banner ||
        banner.contains(target)
      )
        return;
      const focused = target.getBoundingClientRect();
      const overlay = banner.getBoundingClientRect();
      const covered = document.elementFromPoint(
        Math.max(
          0,
          Math.min(innerWidth - 1, (focused.left + focused.right) / 2),
        ),
        Math.max(
          0,
          Math.min(innerHeight - 1, (focused.top + focused.bottom) / 2),
        ),
      );
      if (
        covered &&
        banner.contains(covered) &&
        focused.bottom > overlay.top &&
        focused.top < overlay.bottom &&
        focused.right > overlay.left &&
        focused.left < overlay.right
      ) {
        target.scrollIntoView({
          block: "center",
          inline: "nearest",
          behavior: "instant",
        });
        const moved = target.getBoundingClientRect();
        const atFocus = document.elementFromPoint(
          Math.max(0, Math.min(innerWidth - 1, (moved.left + moved.right) / 2)),
          Math.max(
            0,
            Math.min(innerHeight - 1, (moved.top + moved.bottom) / 2),
          ),
        );
        if (atFocus && banner.contains(atFocus)) {
          const atTop = banner.style.top !== "";
          banner.style.top = atTop ? "" : "24px";
          banner.style.bottom = atTop ? "" : "auto";
        }
      }
    };
    document.addEventListener("focusin", revealFocus);
    return () => document.removeEventListener("focusin", revealFocus);
  }, [loading, value]);
  if (loading || value !== null) return null;

  return (
    <div
      ref={bannerRef}
      role="dialog"
      aria-label="Cookie consent"
      aria-describedby="cookie-consent-description"
      aria-modal="false"
      className="oui-fixed oui-bottom-6 oui-left-1/2 -oui-translate-x-1/2 oui-z-40 oui-w-full oui-max-w-[560px] oui-px-4"
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

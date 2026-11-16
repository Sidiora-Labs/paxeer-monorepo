import React, { useEffect } from "react";
import type { AppProps } from "next/app";
import Link from "next/link";
import { CookieBanner } from "../legal/CookieBanner";
import {
  startConsentErrorReporting,
  useCookieConsent,
} from "../legal/useCookieConsent";
import Head from "next/head";
import "../styles/globals.css";
import "../../../../packages/ui-scaffold/src/surface.css";
import "react-grid-layout/css/styles.css";
import "react-resizable/css/styles.css";
import "../../../../packages/layout-grid/src/styles.css";
export default function WebApp({ Component, pageProps }: AppProps) {
  useCookieConsent();
  useEffect(startConsentErrorReporting, []);
  return (
    <div>
      <Head>
        <meta name="color-scheme" content="light dark" />
      </Head>
      <Component {...pageProps} />
      <nav
        aria-label="Legal"
        className="oui-legal-nav oui-flex oui-flex-wrap oui-justify-center oui-gap-4 oui-p-4 oui-text-2xs oui-text-base-contrast-80"
      >
        <Link href="/privacy" className="hover:oui-text-base-contrast">
          Privacy Policy
        </Link>
        <Link href="/terms" className="hover:oui-text-base-contrast">
          Terms of Service
        </Link>
        <Link href="/cookies" className="hover:oui-text-base-contrast">
          Cookie Settings
        </Link>
      </nav>
      <CookieBanner />
    </div>
  );
}

import React, { useEffect } from "react";
import type { AppProps } from "next/app";
import {
  startConsentErrorReporting,
  useCookieConsent,
} from "../legal/useCookieConsent";
import "../styles/globals.css";
import "react-grid-layout/css/styles.css";
import "react-resizable/css/styles.css";
import "../../../../packages/layout-grid/src/styles.css";
export default function WebApp({ Component, pageProps }: AppProps) {
  useCookieConsent();
  useEffect(startConsentErrorReporting, []);
  return <div>{React.createElement(Component, pageProps)}</div>;
}

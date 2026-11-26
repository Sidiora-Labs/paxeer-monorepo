import React from "react";
import Head from "next/head";

export const APPLICATION_NAME = "Paxeer Xtrades";

export function PageMetadata({ documentTitle }: { documentTitle?: string }) {
  const title = documentTitle
    ? `${documentTitle} | ${APPLICATION_NAME}`
    : APPLICATION_NAME;
  const description = documentTitle
    ? `Read the ${documentTitle} for ${APPLICATION_NAME}.`
    : `${APPLICATION_NAME} provides a perpetual futures trading workspace on the Paxeer Network.`;

  return (
    <Head>
      <title>{title}</title>
      <meta name="application-name" content={APPLICATION_NAME} />
      <meta name="description" content={description} />
      <meta property="og:title" content={title} />
      <meta property="og:description" content={description} />
      <meta property="og:site_name" content={APPLICATION_NAME} />
      <meta name="twitter:card" content="summary" />
      <meta name="twitter:title" content={title} />
      <meta name="twitter:description" content={description} />
      <link rel="icon" type="image/png" href="/brand/paxeer-x.png" />
      <link rel="apple-touch-icon" href="/brand/paxeer-x.png" />
      <meta name="theme-color" content="#06090f" />
    </Head>
  );
}

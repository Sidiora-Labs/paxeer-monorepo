import { useEffect, useLayoutEffect, useState } from "react";
import Head from "next/head";
const useClientLayoutEffect =
  typeof window === "undefined" ? useEffect : useLayoutEffect;

function ClientWorkspace() {
  const [mounted, setMounted] = useState(false);
  const [fontError, setFontError] = useState<Error | null>(null);
  useClientLayoutEffect(() => {
    let active = true;
    Promise.all(
      [400, 600, 700].map((weight) =>
        document.fonts.load(`${weight} 16px DIN2014`),
      ),
    ).then(
      () => {
        if (active) setMounted(true);
      },
      (error: Error) => {
        if (active) setFontError(error);
      },
    );
    return () => {
      active = false;
    };
  }, []);
  if (fontError) throw fontError;
  if (!mounted) {
    return (
      <div role="status" aria-label="Loading workspace" aria-busy="true">
        Loading workspace
      </div>
    );
  }
  const { default: PaxeerNetwork } =
    require("../native/PaxeerNetwork") as typeof import("../native/PaxeerNetwork");
  return <PaxeerNetwork />;
}

export function getServerSideProps() {
  return { props: {} };
}

export default function Page() {
  return (
    <>
      <Head>
        {["regular", "demi", "bold"].map((weight) => (
          <link
            key={weight}
            rel="preload"
            href={`/fonts/DIN2014/din-2014_${weight}.woff2`}
            as="font"
            type="font/woff2"
            crossOrigin="anonymous"
          />
        ))}
        <title>Paxeer Xtrades</title>
        <meta name="application-name" content="Paxeer Xtrades" />
        <link rel="icon" type="image/png" href="/brand/paxeer-x.png" />
        <link rel="apple-touch-icon" href="/brand/paxeer-x.png" />
        <meta name="theme-color" content="#06090f" />
      </Head>
      <ClientWorkspace />
    </>
  );
}

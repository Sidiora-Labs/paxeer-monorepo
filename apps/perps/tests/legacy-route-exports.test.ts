import assert from "node:assert/strict";
import { IncomingMessage, ServerResponse } from "node:http";
import { Socket } from "node:net";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import type { GetServerSidePropsContext } from "next";
import { getPagesPageStaticInfo } from "next/dist/build/analysis/get-page-static-info";
import type { PAGE_TYPES } from "next/dist/lib/page-types";
import * as blockPage from "../src/pages/block/[height]";
import * as txPage from "../src/pages/tx/[hash]";
import * as blockModule from "../src/page-modules/block-detail";
import * as txModule from "../src/page-modules/tx-detail";

const routes = [
  {
    page: "/block/[height]",
    file: "../src/pages/block/[height].tsx",
    param: "height",
    focus: "block",
    entrypoint: blockPage,
    implementation: blockModule,
    identifiers: ["12345", "0", "9007199254740993", `0x${"ab".repeat(32)}`],
  },
  {
    page: "/tx/[hash]",
    file: "../src/pages/tx/[hash].tsx",
    param: "hash",
    focus: "tx",
    entrypoint: txPage,
    implementation: txModule,
    identifiers: [`0x${"CD".repeat(32)}`],
  },
] as const;

async function redirectFor(
  route: (typeof routes)[number],
  params: GetServerSidePropsContext["params"],
  query: GetServerSidePropsContext["query"] = {},
) {
  const socket = new Socket();
  const req = Object.assign(new IncomingMessage(socket), { cookies: {} });
  const context: GetServerSidePropsContext = {
    req,
    res: new ServerResponse(req),
    params,
    query,
    resolvedUrl: route.page,
  };
  try {
    return await route.entrypoint.getServerSideProps(context);
  } finally {
    socket.destroy();
  }
}

for (const route of routes) {
  test(`${route.page} exports the existing page and server redirect handler`, () => {
    assert.equal(route.entrypoint.default, route.implementation.default);
    assert.equal(route.entrypoint.getServerSideProps, route.implementation.getServerSideProps);
    assert.equal(typeof route.entrypoint.getServerSideProps, "function");
  });

  test(`${route.page} is recognized by Next as a server-rendered page`, async () => {
    const info = await getPagesPageStaticInfo({
      pageFilePath: fileURLToPath(new URL(route.file, import.meta.url)),
      nextConfig: {},
      page: route.page,
      pageType: "pages" as PAGE_TYPES.PAGES,
      isDev: false,
    });
    assert.equal(info.getServerSideProps, true);
    assert.equal(info.getStaticProps, false);
  });

  test(`${route.page} preserves identifiers in a temporary explorer redirect`, async () => {
    for (const value of route.identifiers) {
      assert.deepEqual(await redirectFor(route, { [route.param]: value }), {
        redirect: {
          destination: `/explorer?${route.focus}=${value}`,
          permanent: false,
        },
      });
    }
  });

  test(`${route.page} encodes identifier delimiters and Unicode once`, async () => {
    assert.deepEqual(await redirectFor(route, { [route.param]: "a/b?c=d&x=+ #%雪" }), {
      redirect: {
        destination: `/explorer?${route.focus}=a%2Fb%3Fc%3Dd%26x%3D%2B%20%23%25%E9%9B%AA`,
        permanent: false,
      },
    });
  });

  test(`${route.page} uses the first route parameter and ignores query overrides`, async () => {
    assert.deepEqual(
      await redirectFor(
        route,
        { [route.param]: ["first", "second"] },
        { [route.param]: "override", block: "other", tx: "other", next: "https://example.com" },
      ),
      {
        redirect: {
          destination: `/explorer?${route.focus}=first`,
          permanent: false,
        },
      },
    );
  });

  test(`${route.page} retains the plain explorer fallback for missing or empty parameters`, async () => {
    const params: GetServerSidePropsContext["params"][] = [
      undefined,
      {},
      { [route.param]: "" },
      { [route.param]: [] },
      { [route.param]: ["", "second"] },
    ];
    for (const value of params) {
      assert.deepEqual(await redirectFor(route, value, { [route.param]: "query-only" }), {
        redirect: { destination: "/explorer", permanent: false },
      });
    }
  });
}

import { createServer } from "node:http";
import { once, EventEmitter } from "node:events";
import { decodeFunctionData, encodeFunctionResult, numberToHex, type Hex } from "viem";
import { auditMarketConfig, auditMarginTiers } from "./audit-market-config";

export async function auditLoopback() {
  const { PerpsPrecompileABI } = await import("../../src/core/network/abi/PerpsPrecompile");
  const events = new EventEmitter();
  const zeroHash = `0x${"00".repeat(32)}`;
  const state = {
    timestamp: 3599n, height: 41n, blockReads: 0, holdBlocks: false,
    holdGateway: false, gatewayReads: 0, enabled: true,
    contractReads: [] as string[], rpcMethods: [] as string[],
    funding: [] as { positionId: string; userAddress: string }[],
    queries: [] as { query: string; variables: Record<string, string> }[],
  };
  const server = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString());
      res.setHeader("content-type", "application/json");
      if (req.url === "/graphql") {
        state.gatewayReads++;
        state.queries.push(body);
        if (state.holdGateway) return;
        const rows = state.funding.filter((row) => row.positionId === body.variables.positionId &&
          row.userAddress.toLowerCase() === body.variables.userAddress);
        res.end(JSON.stringify({ data: { fundingPayments: rows } }));
        events.emit("gateway");
        return;
      }
      if (req.url !== "/") throw new Error(`Unexpected fixture path ${req.url}`);
      state.rpcMethods.push(body.method);
      let result: unknown;
      if (body.method === "eth_chainId") result = "0x7d";
      else if (body.method === "eth_getBlockByNumber") {
        state.blockReads++;
        if (state.holdBlocks && state.blockReads > 1) return;
        result = {
          number: numberToHex(state.height), timestamp: numberToHex(state.timestamp),
          hash: zeroHash, parentHash: zeroHash, nonce: "0x0000000000000000",
          sha3Uncles: zeroHash, logsBloom: `0x${"00".repeat(256)}`,
          transactionsRoot: zeroHash, stateRoot: zeroHash, receiptsRoot: zeroHash,
          miner: `0x${"00".repeat(20)}`, difficulty: "0x0", totalDifficulty: "0x0",
          extraData: "0x", size: "0x1", gasLimit: "0x1000000", gasUsed: "0x0",
          transactions: [], uncles: [], baseFeePerGas: "0x1",
        };
      } else if (body.method === "eth_call") {
        const decoded = decodeFunctionData({ abi: PerpsPrecompileABI, data: body.params[0].data as Hex });
        state.contractReads.push(decoded.functionName);
        if (decoded.functionName === "getMarketConfig") {
          result = encodeFunctionResult({ abi: PerpsPrecompileABI, functionName: decoded.functionName,
            result: { ...auditMarketConfig, enabled: state.enabled } });
        } else if (decoded.functionName === "getMarginTiers") {
          result = encodeFunctionResult({ abi: PerpsPrecompileABI, functionName: decoded.functionName, result: auditMarginTiers });
        } else throw new Error(`Unexpected contract read ${decoded.functionName}`);
      } else throw new Error(`Unexpected RPC method ${body.method}`);
      res.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, result }));
      if (body.method === "eth_getBlockByNumber") events.emit("block");
    } catch (error) {
      res.statusCode = 500;
      res.end(JSON.stringify({ error: String(error) }));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("Fixture has no TCP address");
  return {
    origin: `http://127.0.0.1:${address.port}`, state, events,
    async close() {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    },
  };
}

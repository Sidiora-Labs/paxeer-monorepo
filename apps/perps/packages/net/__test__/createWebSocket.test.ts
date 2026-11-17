import { once } from "node:events";
import { WebSocketServer } from "ws";
import { WS, WebSocketEvent } from "../src";

test("public websocket opens, sends JSON and closes", async () => {
  const server = new WebSocketServer({ host: "127.0.0.1", port: 0 });
  await once(server, "listening");
  const address = server.address();
  if (typeof address === "string" || !address)
    throw new Error("Expected a TCP listener");
  const connection = once(server, "connection");
  const client = new WS({
    publicUrl: `ws://127.0.0.1:${address.port}`,
    privateUrl: "",
  });
  try {
    const opened = new Promise((resolve) =>
      client.once("status:change", resolve),
    );
    const [peer] = await connection;
    expect(await opened).toEqual({
      type: WebSocketEvent.OPEN,
      isPrivate: false,
      isReconnect: false,
    });
    const received = once(peer, "message");
    client.send({ event: "ping", id: "public" });
    const [message] = await received;
    expect(JSON.parse(message.toString())).toEqual({
      event: "ping",
      id: "public",
    });
    const closed = new Promise((resolve) =>
      client.once("status:change", resolve),
    );
    client.close();
    expect(await closed).toMatchObject({
      type: WebSocketEvent.CLOSE,
      isPrivate: false,
    });
  } finally {
    client.close();
    for (const peer of server.clients) peer.terminate();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
  }
});

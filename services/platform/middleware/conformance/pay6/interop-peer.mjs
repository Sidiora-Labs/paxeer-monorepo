import "./source-loader.mjs";
const { paymentPeer } = await import("./peers.mjs");
const peer = await paymentPeer();
process.stdout.write(`${JSON.stringify({ endpoint: peer.endpoint })}\n`);
process.stdin.resume();
process.stdin.once("data", async () => { await peer.close(); process.stdin.pause(); });

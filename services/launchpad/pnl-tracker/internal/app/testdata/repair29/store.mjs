import { PGlite } from "@electric-sql/pglite";
import { PGLiteSocketServer } from "@electric-sql/pglite-socket";

const db = await PGlite.create(process.argv[2]);
const server = new PGLiteSocketServer({ db, host: "127.0.0.1", port: 0 });
server.addEventListener("error", (event) => {
  console.error(event.detail);
  process.exitCode = 1;
});
await server.start();
console.log(JSON.stringify(server.getServerConn()));
process.once("SIGTERM", async () => {
  await server.stop();
  await db.close();
  process.exit(0);
});

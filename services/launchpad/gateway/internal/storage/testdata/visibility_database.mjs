import { PGlite } from '@electric-sql/pglite'
import { PGLiteSocketServer } from '@electric-sql/pglite-socket'

// Execute the unmodified production SQL in an isolated WASM database. No
// installed database, service configuration, or external endpoint is used.
// PGlite multiplexes one SQL session. Use pgx simple queries so independently
// opened producer and consumer pools do not share cached prepared statements.
const db = await PGlite.create()
const server = new PGLiteSocketServer({ db, host: '127.0.0.1', port: 0, maxConnections: 8 })
await server.start()
console.log(JSON.stringify(`postgres://postgres@${server.getServerConn()}/postgres?sslmode=disable&default_query_exec_mode=simple_protocol&pool_max_conns=1`))
process.stdin.resume()
process.stdin.once('end', async () => {
  await server.stop()
  await db.close()
})

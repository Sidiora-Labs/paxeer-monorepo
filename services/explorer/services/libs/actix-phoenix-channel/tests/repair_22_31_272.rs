use actix_phoenix_channel::{
    ChannelCentral, ChannelConn, ChannelEvent, ChannelHandler, configure_channel_websocket_route,
};
use actix_web::{App, HttpServer};
use serde_json::json;
use std::{net::TcpListener, process::Command, sync::Arc};

struct RoomChannel;

#[async_trait::async_trait]
impl ChannelHandler for RoomChannel {
    async fn join_channel(&self, conn: &ChannelConn, event: ChannelEvent) {
        match event.topic() {
            "room" | "observer" => {
                conn.client()
                    .allow_join(&event, &json!({"joined": true}))
                    .await
            }
            _ => {
                conn.client()
                    .reply_error(&event, &json!({"reason": "forbidden"}))
                    .await
            }
        }
    }

    async fn leave_channel(&self, conn: &ChannelConn, event: ChannelEvent) {
        conn.client()
            .allow_leave(&event, &json!({"left": true}))
            .await;
    }

    async fn incoming_message(&self, conn: &ChannelConn, event: ChannelEvent) {
        match (event.topic(), event.event()) {
            ("observer", "publish") => {
                conn.client().broadcast(("room", "update", event.payload()));
                conn.client()
                    .broadcast(("observer", "barrier", event.payload()));
                conn.client().reply_ok(&event, event.payload()).await;
            }
            ("room", "echo") => conn.client().reply_ok(&event, event.payload()).await,
            _ => {
                conn.client()
                    .reply_error(&event, &json!({"reason": "forbidden"}))
                    .await
            }
        }
    }
}

async fn run_phoenix_case(case: &str) {
    let listener = TcpListener::bind(("127.0.0.1", 0)).unwrap();
    let address = listener.local_addr().unwrap();
    let central = Arc::new(ChannelCentral::new(RoomChannel));
    let server = HttpServer::new(move || {
        let central = central.clone();
        App::new().configure(move |cfg| configure_channel_websocket_route(cfg, central))
    })
    .workers(1)
    .listen(listener)
    .unwrap()
    .run();
    let handle = server.handle();
    let server_task = actix_web::rt::spawn(server);
    let case = case.to_owned();
    let output = tokio::task::spawn_blocking(move || {
        Command::new("node")
            .arg("-e")
            .arg(PHOENIX_EXCHANGE)
            .arg(format!("ws://{address}/socket"))
            .arg(case)
            .output()
    })
    .await
    .unwrap();
    handle.stop(false).await;
    server_task.await.unwrap().unwrap();
    let output =
        output.expect("Node with its built-in WebSocket is required for the Phoenix client");
    assert!(
        output.status.success(),
        "Phoenix client failed: {}\n{}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr),
    );
}

#[actix_web::test]
async fn phoenix_leave_reply_filtered() {
    run_phoenix_case("leave").await;
}

#[actix_web::test]
async fn phoenix_rejected_join_reply() {
    run_phoenix_case("rejected_join").await;
}

#[actix_web::test]
async fn phoenix_rejected_request_reply() {
    run_phoenix_case("rejected_request").await;
}

const PHOENIX_EXCHANGE: &str = r#"
const assert = require('node:assert/strict');
const {createHash} = require('node:crypto');
const {execFileSync} = require('node:child_process');

// Execute the published Phoenix client, including its real serializer and Push consumer.
const source = execFileSync('curl', [
  '--fail', '--silent', '--show-error', '--max-time', '10',
  'https://cdn.jsdelivr.net/npm/phoenix@1.7.21/priv/static/phoenix.cjs.js'
]);
assert.equal(createHash('sha256').update(source).digest('hex'),
  'f11c99e97f00b2de35d750bcf45920175fdf993821820fc867765835af62da2b');
const phoenixModule = {exports: {}};
new Function('module', 'exports', source.toString())(phoenixModule, phoenixModule.exports);
const {Socket} = phoenixModule.exports;
const sockets = [];
const deadline = setTimeout(() => {
  console.error('Phoenix exchange exceeded its deadline');
  process.exit(1);
}, 15000);

function connect() {
  const socket = new Socket(process.argv[1], {
    transport: WebSocket, timeout: 2000, heartbeatIntervalMs: 60000,
    rejoinAfterMs: () => 60000, reconnectAfterMs: () => 60000
  });
  const messages = [];
  socket.onMessage(message => messages.push(message));
  sockets.push(socket);
  return new Promise((resolve, reject) => {
    socket.onOpen(() => resolve({socket, messages}));
    socket.onError(error => reject(new Error(`WebSocket error: ${error}`)));
    socket.connect();
  });
}

function receive(push, status) {
  return new Promise((resolve, reject) => {
    for (const result of ['ok', 'error', 'timeout']) {
      push.receive(result, response => {
        if (result === status) resolve(response);
        else reject(new Error(`expected ${status}, received ${result}: ${JSON.stringify(response)}`));
      });
    }
  });
}

function nextMessage(socket, predicate) {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      socket.off([binding]);
      reject(new Error('server did not deliver the expected wire message'));
    }, 2000);
    const binding = socket.onMessage(message => {
      if (predicate(message)) {
        clearTimeout(timeout);
        socket.off([binding]);
        resolve(message);
      }
    });
  });
}

function assertReply(client, channel, push, status, response) {
  const replies = client.messages.filter(message => message.ref === push.ref);
  assert.equal(replies.length, 1, 'the original request ref must receive exactly one wire reply');
  assert.deepEqual(replies[0], {
    join_ref: channel.joinRef(), ref: push.ref, topic: channel.topic,
    event: 'phx_reply', payload: {status, response}
  });
}

async function join(client, topic) {
  const channel = client.socket.channel(topic, {});
  const push = channel.join();
  assert.deepEqual(await receive(push, 'ok'), {joined: true});
  assertReply(client, channel, push, 'ok', {joined: true});
  assert.equal(channel.isJoined(), true);
  return channel;
}

async function exchange() {
  const client = await connect();
  if (process.argv[2] === 'rejected_join') {
    const channel = client.socket.channel('private', {});
    let lifecycleErrors = 0;
    channel.onError(() => lifecycleErrors++);
    const push = channel.join();
    assert.deepEqual(await receive(push, 'error'), {reason: 'forbidden'});
    assertReply(client, channel, push, 'error', {reason: 'forbidden'});
    assert.equal(channel.isJoined(), false);
    assert.equal(lifecycleErrors, 0, 'a request refusal is not phx_error');
    return;
  }
  const room = await join(client, 'room');
  if (process.argv[2] === 'rejected_request') {
    let lifecycleErrors = 0;
    room.onError(() => lifecycleErrors++);
    const push = room.push('restricted', {operation: 'write'});
    assert.deepEqual(await receive(push, 'error'), {reason: 'forbidden'});
    assertReply(client, room, push, 'error', {reason: 'forbidden'});
    assert.equal(lifecycleErrors, 0);
    assert.equal(room.isJoined(), true);
    const echo = room.push('echo', {message: 'still joined'});
    assert.deepEqual(await receive(echo, 'ok'), {message: 'still joined'});
    assertReply(client, room, echo, 'ok', {message: 'still joined'});
    return;
  }
  assert.equal(process.argv[2], 'leave');
  const observer = await join(client, 'observer');
  const witness = await connect();
  await join(witness, 'room');
  async function publish(marker, expectRoom) {
    const observed = [
      nextMessage(client.socket, m => m.topic === 'observer' && m.event === 'barrier' && m.payload.marker === marker),
      nextMessage(witness.socket, m => m.topic === 'room' && m.event === 'update' && m.payload.marker === marker)
    ];
    if (expectRoom) observed.push(nextMessage(client.socket,
      m => m.topic === 'room' && m.event === 'update' && m.payload.marker === marker));
    const push = observer.push('publish', {marker});
    await Promise.all([...observed, receive(push, 'ok')]);
    assertReply(client, observer, push, 'ok', {marker});
  }
  await publish('before leave', true);
  const wireLeave = nextMessage(client.socket,
    m => m.topic === 'room' && m.event === 'phx_reply' && m.ref !== room.joinRef());
  const leave = room.leave();
  await receive(leave, 'ok');
  await wireLeave;
  // Phoenix may acknowledge leave locally; require the server's actual wire acknowledgement.
  assertReply(client, room, leave, 'ok', {left: true});
  assert.equal(room.isClosed(), true);
  const boundary = client.messages.length;
  await publish('after leave', false);
  assert.equal(client.messages.slice(boundary).some(m => m.topic === 'room'), false,
    'the observer barrier follows the room broadcast in the same broadcast queue');
  assert.equal(witness.messages.some(m => m.topic === 'room' && m.event === 'update' &&
    m.payload.marker === 'after leave'), true, 'the broadcast must reach a still subscribed client');
}

exchange().then(() => {
  clearTimeout(deadline);
  sockets.forEach(socket => socket.disconnect());
  process.exit(0);
}, error => {
  console.error(error);
  process.exit(1);
});
"#;

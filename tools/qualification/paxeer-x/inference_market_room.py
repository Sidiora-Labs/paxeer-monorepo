#!/usr/bin/env python3
"""A bounded, private proof of a native inference room on a disposable chain.

Run keys, native genesis, room terms and signed writes are produced in the
private evidence directory. Program payloads come from the maintained Python
SDK or the layerx-client inference_room_writes example. No public node is used.
"""

import argparse
import base64
from concurrent.futures import ThreadPoolExecutor
import hashlib
import http.client
import json
import os
from pathlib import Path
import py_compile
import re
import shutil
import socket
import sqlite3
import ssl
import string
import subprocess
import sys
import tempfile
import time
import uuid
from types import SimpleNamespace
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(Path(__file__).parent))
import agent_operation_envelope as envelope
from fixtures.merged_runtime_fixture import RuntimeFixture, RootRpc, ChainWriter, reserve_ports
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed
from layerx_sdk.evm_transaction import Type2Transaction, encode_call, decode_abi
from layerx_sdk import account_derivation as accounts
from layerx_sdk.program_lifecycle import NativeProgramDeploy, NativeProgramUpgrade
from layerx_sdk.native_program_call import decode_native_program_call
from fixtures.xweb_native_evm import sign_transaction
from fixtures.hosted_delivery_fixture import CaMaterial
from fixtures.receipt_authority_fixture import der_pair, https
from cryptography.hazmat.primitives.serialization import PrivateFormat, NoEncryption
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec, utils
from cryptography import x509

SCHEMA = 'paxeer-x.inference-room.v1'
TASK = '22.38.36'
OWN = (
    'tools/qualification/paxeer-x/inference_market_room.py',
    'tools/qualification/paxeer-x/agent_operation_envelope.py',
    'tools/qualification/paxeer-x/fixtures/merged_runtime_fixture.py',
    'services/agent/crates/layerx-client/Cargo.toml',
    'services/agent/crates/layerx-client/examples/inference_room_writes.rs',
)
WRITE_PHASES = ('deploy', 'open_market', 'open_room', 'enrol_peers',
                'fund_ask_stream', 'post_ask')
# The host retains embedding and head; the two workers submit the layer bids.
# The peer package does not submit chain transactions. Negotiation and lap
# settlement are daemon-owned writes, observed without replacing their producer.
WRITE_OWNERS = {
    'deploy': 'driver: Python SDK NativeProgramDeploy',
    'open_market': 'driver: layerx-client CREATE',
    'open_room': 'driver: layerx-client OpenRoom',
    'enrol_peers': 'driver: native peer enrolment and quotes',
    'fund_ask_stream': 'asker: Python SDK streamOpenForAsk',
    'post_ask': 'asker: layerx-client PostAsk; daemon: RecordDeal',
}
PROGRAMS = '0x' + '00' * 18 + '101d'
LEDGER = '0x' + '00' * 18 + '101a'
PAY = '0x' + '00' * 18 + '101b'
RESOURCES = [1_000_000, 16_777_216, 1_048_576, 1_048_576, 64, 1_048_576, 4096]
OWNER_NAMESPACE_BYTES = 4 * 1_048_576
OWNER_RENT_BATCHES = 64
# integer_guest from layerx-programs-runtime/src/market_tests.rs: a compute(i32)
# guest accepted by the ABI 2 validator before the AI market guest is installed.
ABI2_GUEST = bytes.fromhex(
    '0061736d0100000001060160017f017f03020100070b0107636f6d707574650000'
    '0a0c010a00200041026a41036a0b')
require = envelope.require
save = envelope.write_private
load = envelope.load_private


# The native transport carries the package's real bounded wire frames over
# private loopback sockets. DealClient, SignalClient and LapTracker own protocol
# validation, execution, acknowledgements and countersignatures.
NATIVE = r'''
import assert from 'node:assert/strict';
import {createHash, timingSafeEqual} from 'node:crypto';
import {createServer as tcpServer, connect} from 'node:net';
import {createServer as httpServer} from 'node:http';
import {readFile, writeFile, mkdir, access} from 'node:fs/promises';
import {once} from 'node:events';
import WebSocket from '@PEER@/node_modules/ws/wrapper.mjs';
import {PeerIdentity, manifestDigest} from '@PEER@/src/identity.mjs';
import {SignalClient} from '@PEER@/src/transport.mjs';
import {DealClient, dealDigest, encodeActivation, decodeActivation} from '@PEER@/src/deal.mjs';
import {LapTracker, lapReceiptBytes, lapSigningDigest, lapCountersignDigest} from '@PEER@/src/laps.mjs';
import {encodeFrame, decodeFrame, MAX_FRAME_BYTES} from '@PEER@/src/wire.mjs';
import {scriptedModelSource, scriptedEmbedding, scriptedToken} from '@PEER@/tests/scripted_model.mjs';
import {ScriptedRuntime} from '@PEER@/tests/scripted_model.mjs';

const [mode, configPath, peerNumber] = process.argv.slice(2);
const hex = value => Buffer.from(value).toString('hex');
const sha = value => createHash('sha256').update(value).digest();
const fatal = error => { console.error(error.stack ?? error); process.exit(1); };
process.on('uncaughtException', fatal);
process.on('unhandledRejection', fatal);
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
class NativeWebSocket extends WebSocket {
  constructor(url) {
    super(url);
    let heartbeat;
    this.once('open', () => {
      heartbeat = setInterval(() => {
        if (this.readyState === WebSocket.OPEN) this.ping();
      },1000);
      heartbeat.unref();
    });
    this.once('close', () => clearInterval(heartbeat));
  }
}
async function save(path, value) {
  await writeFile(path, JSON.stringify(value) + '\n', {flag:'wx', mode:0o600});
}
async function read(path) { return JSON.parse(await readFile(path, 'utf8')); }
async function waitFor(predicate, seconds=20) {
  const until = Date.now() + seconds * 1000;
  while (Date.now() < until) {
    const result = await predicate();
    if (result) return result;
    await sleep(25);
  }
  throw new Error('Native room deadline');
}
function packet(socket, value) {
  const body = Buffer.from(JSON.stringify(value));
  assert(body.length > 0 && body.length <= 163840);
  const head = Buffer.alloc(4); head.writeUInt32BE(body.length);
  return new Promise((resolve, reject) => socket.write(Buffer.concat([head,body]),
    error => error ? reject(error) : resolve()));
}
function packets(socket, handler) {
  let pending = Buffer.alloc(0), queue = Promise.resolve();
  socket.on('error', fatal);
  socket.on('data', bytes => {
    pending = Buffer.concat([pending,bytes]);
    while (pending.length >= 4) {
      const n = pending.readUInt32BE();
      assert(n > 0 && n <= 163840);
      if (pending.length < n + 4) break;
      const value = JSON.parse(pending.subarray(4,n+4));
      pending = pending.subarray(n+4);
      queue = queue.then(() => handler(value)).catch(fatal);
    }
    assert(pending.length <= 163844);
  });
}
async function rpc(url, method, params) {
  const response = await fetch(url, {method:'POST', redirect:'error',
    headers:{'content-type':'application/json'},
    body:JSON.stringify({jsonrpc:'2.0',id:1,method,params}), signal:AbortSignal.timeout(10000)});
  assert.equal(response.status,200);
  const raw = await response.text(); assert(Buffer.byteLength(raw) <= 1048576);
  const value = JSON.parse(raw);
  assert.equal(value.jsonrpc,'2.0'); assert.equal(value.id,1);
  assert(!('error' in value) && 'result' in value, JSON.stringify(value));
  return value.result;
}
function text(token) { return `[${token}]`; }
if (mode === 'model') {
  const source = await scriptedModelSource();
  await mkdir(configPath, {mode:0o700});
  for (const [name,bytes] of source.resources)
    await writeFile(configPath + name, bytes, {flag:'wx',mode:0o600});
  const cache = source.model.createCache('serial-reference');
  let token = 5;
  await source.model.runActivation(scriptedEmbedding(token), 0, cache);
  const tokens = [];
  for (let i=0; i<4; i++) {
    token = scriptedToken(await source.model.runActivation(scriptedEmbedding(token),i+1,cache));
    tokens.push({token_id:token,text:text(token)});
  }
  source.model.dropCache(cache); source.model.close();
  await save(configPath + '/reference.json', {model_digest:hex(source.digest),
    prompt:'5', prompt_tokens:1, tokens});
} else {
  const c = await read(configPath);
  class NativeMesh extends EventTarget {
    sequence = 0;
    async open(peer) {
      this.socket = connect(c.router_port,'127.0.0.1');
      await once(this.socket,'connect');
      const ready = once(this,'ready');
      packets(this.socket, message => {
        if (message.type === 'connected') { this.dispatchEvent(new Event('ready')); return; }
        assert.equal(message.type,'frame');
        const bytes = Buffer.from(message.frame,'hex'); assert(bytes.length <= MAX_FRAME_BYTES);
        this.dispatchEvent(new CustomEvent('frame', {detail:{peerKey:message.from,frame:decodeFrame(bytes)}}));
      });
      await packet(this.socket,{type:'connect',peer:peer.peer_key,token:peer.transport_token});
      await ready;
    }
    send(to,kind,payload) {
      const frame = encodeFrame({kind,roomId:c.room.room_id,sequence:this.sequence++,payload});
      return packet(this.socket,{type:'frame',to,frame:hex(frame)});
    }
    close() { this.socket?.destroy(); this.dispatchEvent(new Event('close')); }
  }
  if (mode === 'peer') {
    const index = Number(peerNumber), peer = c.peers[index];
    assert(index === 0 || index === 1);
    await access(peer.identity);
    const identity = await PeerIdentity.open({mode:'native',path:peer.identity});
    assert.equal(identity.peerKey,peer.peer_key);
    const manifest = new Uint8Array(await readFile(peer.manifest));
    await manifestDigest(manifest); assert.equal(manifest.at(-1),2);
    const mesh = new NativeMesh(); await mesh.open(peer);
    async function connectSignal() {
      const deadline = Date.now() + 20000;
      for (let retry=0;retry<8;retry++) {
        const remaining = deadline - Date.now();
        assert(remaining > 0,'Signalling admission deadline');
        const signal = new SignalClient(c.signal_url,{roomId:c.room.room_id,identity,manifest,
          WebSocket:NativeWebSocket,timeoutMs:Math.min(10000,remaining)});
        try { return {signal,offer:await signal.connect()}; }
        catch(error) {
          signal.close();
          if (error.code !== 'chain_unavailable' || retry === 7) throw error;
          await sleep(250);
        }
      }
      throw new Error('Signalling admission deadline');
    }
    const {signal,offer:initialOffer} = await connectSignal();
    signal.addEventListener('error', ({detail}) => fatal(detail));
    signal.addEventListener('close', () => queueMicrotask(() => fatal(new Error('Signalling closed during room proof'))));
    let client, queue = Promise.resolve();
    const offered = Promise.resolve(initialOffer);
    signal.addEventListener('frame', ({detail:frame}) => {
      queue = queue.then(async () => {
        if (frame.type === 'deal') {
          assert(!client);
          const deal = frame.deal, offer = (await offered).offer;
          assert.equal(hex(deal.room_id),c.room.room_id);
          assert.equal(hex(deal.ask_id),c.ask_id);
          assert.equal(hex(deal.model_digest),c.model_digest);
          assert.equal(hex(deal.runtime_digest),c.room.runtime_digest);
          assert.equal(hex(deal.plan.embedding_host),c.room.host_peer_key);
          assert.equal(hex(deal.plan.head_host),c.room.host_peer_key);
          assert.equal(deal.plan.layer_count,4);
          assert.equal(deal.plan.layers.length,2);
          assert.equal(String(deal.price),c.price);
          const expected = new Set(c.peers.map(p => p.peer_key));
          for (const split of deal.plan.layers) assert(expected.delete(hex(split.peer_key)));
          assert.equal(expected.size,0);
          const tracker = new LapTracker({identity,deal,hostMeterKey:c.host_meter_key});
          client = new DealClient({roomId:c.room.room_id, identity, offer,
            runtime:new ScriptedRuntime(), mesh, hostPeerKey:c.room.host_peer_key,
            modelSource:{manifest_url:c.model_url + '/manifest.json'}, lapTracker:tracker,
            countersign:ack => signal.countersignLap(ack),
            acknowledge:ack => signal.acknowledgeDeal(ack),
            validateDeal:async value => {
              assert.equal(await rpc(c.rpc_url,'eth_chainId',[]),'0x7d');
              const room = await rpc(c.rpc_url,'px_getInferenceRoom',[c.room.room_id]);
              assert.equal(room.state,0); assert.equal(room.model_digest,c.model_digest);
              assert(room.open_asks.includes(hex(value.ask_id)));
              let end = 0;
              for (const split of value.plan.layers) {
                assert(room.open_bids.includes(hex(split.bid_id)));
                assert.equal(split.first_layer,end); assert(split.end_layer > end); end = split.end_layer;
                const owner = await rpc(c.rpc_url,'px_getInferencePeer',[hex(split.owner)]);
                assert.equal(owner.delegate,hex(split.peer_key));
                assert.equal(owner.metadata,hex(split.manifest_digest)); assert.equal(owner.enrolled,true);
              }
              assert.equal(end,4); return true;
            }});
          client.addEventListener('error',({detail}) => fatal(detail));
          client.addEventListener('lap-refusal',({detail}) => fatal(new Error(JSON.stringify(detail))));
          client.addEventListener('lap',({detail}) => {
            const path = c.run + `/peer${index}-countersign.json`;
            void save(path,detail).catch(async error => {
              if (error.code !== 'EEXIST') throw error;
              assert.deepEqual(await read(path),detail);
            }).catch(fatal);
          });
          await client.handleFrame(frame);
          await save(c.run + `/peer${index}-deal.json`,deal);
        } else if (['deal_live','lap_receipt'].includes(frame.type)) {
          assert(client); await client.handleFrame(frame);
          if (frame.type === 'deal_live') {
            assert.equal(client.state,'ready');
            await save(c.run + `/peer${index}-live.json`,frame);
          }
        }
      }).catch(fatal);
    });
    await offered;
    await save(c.run + `/peer${index}-offered.json`,{peer_key:identity.peerKey});
  } else if (mode === 'host') {
    const sockets = new Map(), ranges = new Map(), outputs = new Map();
    let sequence = 0;
    const router = tcpServer(socket => {
      let owner, expectedSequence = 0;
      packets(socket, async message => {
        if (!owner) {
          assert.equal(message.type,'connect');
          const peer = c.peers.find(p => p.peer_key === message.peer); assert(peer);
          const actual = Buffer.from(message.token ?? '');
          const wanted = Buffer.from(peer.transport_token);
          assert(actual.length === wanted.length && timingSafeEqual(actual,wanted));
          assert(!sockets.has(peer.peer_key)); owner = peer.peer_key;
          sockets.set(owner,socket); await packet(socket,{type:'connected'}); return;
        }
        assert.equal(message.type,'frame');
        const bytes = Buffer.from(message.frame,'hex'), frame = decodeFrame(bytes);
        assert.equal(frame.roomId,c.room.room_id); assert.equal(frame.sequence,expectedSequence++);
        if (frame.kind === 'activation') {
          const activation = decodeActivation(frame.payload);
          const key = owner + ':' + activation.requestId;
          const values = ranges.get(key) ?? []; values.push(Buffer.from(frame.payload)); ranges.set(key,values);
          if (message.to === c.room.host_peer_key) {
            outputs.set(activation.requestId + ':' + activation.position,activation); return;
          }
        }
        assert(sockets.has(message.to));
        await packet(sockets.get(message.to),{type:'frame',from:owner,frame:message.frame});
      });
      socket.on('close',() => { if (owner) sockets.delete(owner); });
    });
    router.listen(c.router_port,'127.0.0.1'); await once(router,'listening');
    const resources = new Set(['manifest.json',...Array.from({length:4},(_,i) => `layer-${i}.json`)]);
    const models = httpServer(async (request,response) => {
      try {
        const name = request.url.slice(1);
        if (request.method !== 'GET' || !resources.has(name)) { response.writeHead(404).end(); return; }
        const body = await readFile(c.model_directory + '/' + name);
        response.writeHead(200,{'content-type':'application/json','content-length':body.length}); response.end(body);
      } catch (error) { fatal(error); }
    });
    models.listen(c.model_port,'127.0.0.1'); await once(models,'listening');
    const completion = tcpServer(socket => {
      let used = false;
      packets(socket, async request => {
        assert(!used); used = true;
        const deal = await read(c.run + '/peer0-deal.json');
        assert.deepEqual(deal,await read(c.run + '/peer1-deal.json'));
        assert.equal(hex(request.deal_id),hex(deal.deal_id));
        assert.equal(hex(request.ask_id),c.ask_id); assert.equal(hex(request.model_digest),c.model_digest);
        assert.equal(request.next_lap_index,0); assert.equal(request.cumulative_tokens,0);
        assert.equal(request.max_generated_tokens,4); assert(request.token_budget >= 5);
        assert.deepEqual(request.messages,[{role:'user',content:'5'}]);
        const id = hex(request.request_id), binding = await dealDigest(deal);
        const route = deal.plan.layers.map(split => hex(split.peer_key));
        assert(route.every(key => sockets.has(key)));
        async function activation(token, position) {
          const payload = encodeActivation({requestId:id,digest:binding,position,layer:0,values:scriptedEmbedding(token)});
          const frame = encodeFrame({kind:'activation',roomId:c.room.room_id,sequence:sequence++,payload});
          await packet(sockets.get(route[0]),{type:'frame',from:c.room.host_peer_key,frame:hex(frame)});
          const key = id + ':' + position;
          const output = await waitFor(() => outputs.get(key)); outputs.delete(key);
          assert.equal(output.layer,4); assert.equal(hex(output.digest),hex(binding)); return output.values;
        }
        let token = 5; await activation(token,0);
        const tokens = [];
        for (let i=0;i<4;i++) { token = scriptedToken(await activation(token,i+1)); tokens.push({token_id:token,text:text(token)}); }
        const rangeDigests = route.map(key => {
          const values = ranges.get(key + ':' + id); assert.equal(values.length,5);
          return Array.from(sha(Buffer.concat(values)));
        });
        const response = await fetch(c.completion_url + '/internal/inference/laps',{
          method:'POST',redirect:'error',headers:{'content-type':'application/json',authorization:'Bearer ' + c.agent_bearer},
          body:JSON.stringify({deal_id:deal.deal_id,lap_index:0,prompt_tokens:1,generated_tokens:4,
            layer_range_digests:rangeDigests,deal_end:true}),signal:AbortSignal.timeout(10000)});
        assert.equal(response.status,200); const issued = await response.json();
        await lapCountersignDigest(issued);
        assert.equal(hex(issued.deal_digest),hex(binding));
        assert.deepEqual(issued.host_meter_key,c.host_meter_key);
        assert.deepEqual(issued.receipt.deal_id,deal.deal_id);
        assert.equal(issued.receipt.lap_index,0);
        assert.equal(issued.receipt.prompt_tokens,1);
        assert.equal(issued.receipt.generated_tokens,4);
        assert.equal(issued.receipt.cumulative_tokens,5);
        assert.deepEqual(issued.receipt.layer_range_digests,rangeDigests);
        const receipt = await waitFor(async () => {
          const session = await read(c.journal + '/lap-' + hex(deal.deal_id) + '.json');
          const match = session.receipts.find(r => r.lap_index === 0);
          if (match) return match;
          try { return (await read(c.journal + '/lap-' + hex(deal.deal_id) + '.batch-0.json')).receipts[0]; }
          catch (error) { if(error.code !== 'ENOENT') throw error; }
        });
        assert.equal(receipt.peer_countersignatures,3);
        assert.deepEqual(receipt.layer_range_digests,rangeDigests);
        const key = await crypto.subtle.importKey('raw',Uint8Array.from(c.host_meter_key),'Ed25519',false,['verify']);
        assert(await crypto.subtle.verify('Ed25519',key,Uint8Array.from(receipt.host_signature),await lapSigningDigest(receipt)));
        await save(c.run + '/model-output.json',{tokens,receipt,range_digests:rangeDigests});
        await packet(socket,{type:'lap',request_id:request.request_id,receipt:Array.from(lapReceiptBytes(receipt)),tokens});
        await packet(socket,{type:'done',request_id:request.request_id,reason:'length'}); socket.end();
      });
    });
    completion.listen(c.completion_socket); await once(completion,'listening');
    await save(c.run + '/host-ready.json',{native_transport:'loopback-tcp',model_backend:'scripted-cpu'});
  } else throw new Error('Unknown native participant');
}
'''

CUSTODY_SHARES = r'''
package main

import (
    "encoding/hex"
    "encoding/json"
    "fmt"
    "math/big"
    "os"
    "github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/server"
    dealer "github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/tss/dealershare"
)

func main() {
    var input struct { Seed, Secret string; Members []string }
    if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil { panic(err) }
    seed, err := hex.DecodeString(input.Seed); if err != nil { panic(err) }
    secret, ok := new(big.Int).SetString(input.Secret, 16); if !ok { panic("secp256k1 scalar") }
    ed, err := dealer.Ed25519ScalarFromSeed(seed); if err != nil { panic(err) }
    output := map[string]any{}
    for _, item := range []struct { Name string; Curve dealer.Curve; Secret *big.Int }{
        {"ed25519", dealer.Ed25519, ed}, {"secp256k1", dealer.Secp256k1, secret},
    } {
        shares, public, err := dealer.Split(item.Curve, item.Secret, input.Members)
        if err != nil { panic(err) }
        var encoded []byte
        if item.Curve == dealer.Ed25519 { encoded, err = dealer.EncodeEd25519(public) } else {
            encoded = append([]byte{4}, public.GetX().FillBytes(make([]byte,32))...)
            encoded = append(encoded, public.GetY().FillBytes(make([]byte,32))...)
        }
        if err != nil { panic(err) }
        byNode := map[string]any{}
        for _, share := range shares { byNode[share.ParticipantID] = server.EncodeBundle(share) }
        output[item.Name] = map[string]any{"public_key":hex.EncodeToString(encoded), "shares":byNode}
    }
    raw, err := json.Marshal(output); if err != nil { panic(err) }; fmt.Println(string(raw))
}
'''


CHAIN_EVIDENCE = r'''
package main

import (
    "encoding/hex"
    "encoding/json"
    "fmt"
    "os"
    "github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
    ledgerTypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
    tx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
    evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
    abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
    "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
    cmtjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
    tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func main() {
    if len(os.Args) == 4 && os.Args[1] == "bank-mirror" {
        account, err := hex.DecodeString(os.Args[2]); must(err)
        asset, err := hex.DecodeString(os.Args[3]); must(err)
        if len(account) != 32 || len(asset) != 32 { panic("account and asset identifiers required") }
        out := map[string]string{"address":hex.EncodeToString(lxphost.BankAddress([32]byte(account))),
            "denom":ledgerTypes.MirrorDenom([32]byte(asset))}
        must(json.NewEncoder(os.Stdout).Encode(out)); return
    }
    var in struct {
        Signed json.RawMessage `json:"signed"`
        Validators json.RawMessage `json:"validators"`
        Transactions json.RawMessage `json:"transactions"`
        Results json.RawMessage `json:"results"`
        TransactionHash string `json:"transaction_hash"`
    }
    must(json.NewDecoder(os.Stdin).Decode(&in))
    var signed struct { Header tmtypes.Header `json:"header"`; Commit tmtypes.Commit `json:"commit"` }
    must(cmtjson.Unmarshal(in.Signed, &signed))
    var validators []*tmtypes.Validator
    must(cmtjson.Unmarshal(in.Validators, &validators))
    if len(validators) == 0 { panic("validators required") }
    set := tmtypes.NewValidatorSet(validators)
    vp, err := set.ToProto(); must(err)
    vp.TotalVotingPower = set.TotalVotingPower()
    vb, err := vp.Marshal(); must(err)
    hb, err := signed.Header.ToProto().Marshal(); must(err)
    cb, err := signed.Commit.ToProto().Marshal(); must(err)
    out := map[string]any{"signed_header":hex.EncodeToString(hb), "commit":hex.EncodeToString(cb), "validator_set":hex.EncodeToString(vb)}
    if len(in.Transactions) != 0 {
        var txs tmtypes.Txs
        var results []*abci.ExecTxResult
        must(cmtjson.Unmarshal(in.Transactions, &txs))
        must(cmtjson.Unmarshal(in.Results, &results))
        if len(txs) == 0 || len(txs) != len(results) { panic("transaction and result count required") }
        index := -1
        for i, encoded := range txs {
            var raw tx.TxRaw
            if raw.Unmarshal(encoded) != nil { continue }
            var body tx.TxBody
            if body.Unmarshal(raw.BodyBytes) != nil { continue }
            for _, any := range body.Messages {
                if any.TypeUrl != "/paxprotocol.paxchain.evm.MsgEVMTransaction" { continue }
                var message evmtypes.MsgEVMTransaction
                must(message.Unmarshal(any.Value))
                transaction, _ := message.AsTransaction()
                if transaction != nil && transaction.Hash().Hex() == in.TransactionHash {
                    if index != -1 { panic("duplicate deployment transaction") }
                    index = i
                }
            }
        }
        if index < 0 { panic("deployment transaction absent from committed block") }
        leaves, err := abci.MarshalTxResults(results); must(err)
        root, proofs := merkle.ProofsFromByteSlices(leaves)
        if string(root) != string(signed.Header.LastResultsHash) { panic("committed result root mismatch") }
        txProof := txs.Proof(index)
        out["transaction"] = hex.EncodeToString(txs[index])
        out["transaction_proof"] = proof(txProof.Proof)
        out["result_proof"] = proof(*proofs[index])
        result := results[index]
        out["result"] = map[string]any{"code":result.Code, "data":hex.EncodeToString(result.Data), "gas_wanted":result.GasWanted, "gas_used":result.GasUsed}
    }
    encoded, err := json.Marshal(out); must(err); fmt.Println(string(encoded))
}
func must(err error) { if err != nil { panic(err) } }
func proof(p merkle.Proof) map[string]any {
    aunts := make([]string,len(p.Aunts)); for i, aunt := range p.Aunts { aunts[i] = hex.EncodeToString(aunt) }
    return map[string]any{"total":p.Total,"index":p.Index,"leaf_hash":hex.EncodeToString(p.LeafHash),"aunts":aunts}
}
'''


def fingerprint():
    paths = ('apps/inference-peer', 'services/agent/crates', 'services/agent/Cargo.toml',
             'services/agent/Cargo.lock', 'services/platform/hosted',
             'services/platform/Cargo.toml', 'services/platform/Cargo.lock',
             'services/wallet/attestor', 'services/agent/sdk/python',
             'programs', 'src', 'include', 'node', 'rpc', 'precompiles/layerxpay',
             'modules/layerxpay', *OWN)
    names = envelope.capture(['git', 'ls-files', '-z', '--cached', '--others',
                              '--exclude-standard', '--', *paths]).split('\0')
    names = sorted(set(filter(None, names)) | set(OWN))
    digest = hashlib.sha256()
    for name in names:
        path = ROOT / name
        digest.update(name.encode() + b'\0')
        digest.update(os.fsencode(os.readlink(path)) if path.is_symlink() else path.read_bytes())
        digest.update(b'\0')
    return digest.hexdigest()


def artifact_base():
    builder = os.environ.get('PAXEER_X_BUILDER_ID', '')
    require(builder.strip() == builder and builder, 'PAXEER_X_BUILDER_ID is required')
    tag = hashlib.sha256((str(ROOT) + '\0' + builder).encode()).hexdigest()[:24]
    return envelope.private_dir('/var/tmp/paxeer-x-inference-room-' + tag, 'room artifacts')


def active_build():
    base = artifact_base()
    active = load(base / 'active.json', 'active room build')
    require(re.fullmatch('[0-9a-f]{32}', active['build_id']), 'room build identifier')
    return base / active['build_id']


def begin_build():
    directory = artifact_base() / uuid.uuid4().hex
    directory.mkdir(mode=0o700)
    save(directory / 'start.json', {'schema': SCHEMA, 'task': TASK, 'source_digest': fingerprint(),
                                   'builder': os.environ['PAXEER_X_BUILDER_ID'],
                                   'source_revision': envelope.capture(['git', 'rev-parse', 'HEAD'])})
    pending = directory.parent / ('active-' + directory.name + '.json')
    save(pending, {'build_id': directory.name})
    pending.replace(directory.parent / 'active.json')
    print('INFERENCE_ROOM_BUILD ' + str(directory), flush=True)


def build():
    directory = active_build()
    start = load(directory / 'start.json', 'room build start')
    require(start['source_digest'] == fingerprint(), 'room sources changed during build')
    require(start['builder'] == os.environ['PAXEER_X_BUILDER_ID'], 'room builder changed')
    metadata = json.loads(envelope.capture(['cargo', 'metadata', '--locked', '--no-deps',
                                           '--format-version=1', '--manifest-path', 'services/agent/Cargo.toml']))
    agent = Path(metadata['target_directory']) / 'debug/layerx-agentd'
    require(agent.is_file() and os.access(agent, os.X_OK), 'declared daemon build artifact missing')
    require((ROOT / 'apps/inference-peer/node_modules').is_dir(), 'declared peer package install missing')
    native = directory / 'native.mjs'
    descriptor = os.open(native, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, 'w') as stream:
        stream.write(NATIVE.replace('@PEER@', (ROOT / 'apps/inference-peer').as_uri()))
    encoder = Path(metadata['target_directory']) / 'debug/examples/inference_room_writes'
    platform = json.loads(envelope.capture(['cargo', 'metadata', '--locked', '--no-deps',
                                            '--format-version=1', '--manifest-path', 'services/platform/Cargo.toml']))
    platform_target = Path(platform['target_directory']) / 'debug'
    support = {name: str(platform_target / name) for name in
               ('layerx-ai-source', 'layerx-indexer', 'layerx-gateway', 'layerx-runtime-clock',
                'layerx-receipt-authority')}
    support['attestor'] = str(directory / 'attestor')
    support['custody-shares'] = str(directory / 'custody-shares')
    support['chain-evidence'] = str(directory / 'chain-evidence')
    share_source = directory / 'custody-source'
    share_source.mkdir(mode=0o700)
    module = 'github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor'
    protected_bytes(share_source / 'go.mod', (
        'module ' + module + '/inference-room\n\ngo 1.25.8\n\nrequire ' + module + ' v0.0.0\n'
        + 'replace ' + module + ' => ' + str(ROOT / 'services/wallet/attestor') + '\n'
        + 'replace github.com/Sidiora-Labs/Paxeer-X-Network => ' + str(ROOT) + '\n').encode())
    protected_bytes(share_source / 'main.go', CUSTODY_SHARES.encode())
    evidence_source = directory / 'evidence-source'
    evidence_source.mkdir(mode=0o700)
    root_module = json.loads(envelope.capture(['go', 'mod', 'edit', '-json']))
    replacements = []
    for item in root_module.get('Replace', []):
        old, new = item['Old'], item['New']
        target = str((ROOT / new['Path']).resolve()) if not new.get('Version') else new['Path']
        replacements.append('replace ' + old['Path'] + (' ' + old['Version'] if old.get('Version') else '')
                            + ' => ' + target + (' ' + new['Version'] if new.get('Version') else '') + '\n')
    protected_bytes(evidence_source / 'go.mod', (share_source / 'go.mod').read_bytes()
                    + ''.join(replacements).encode())
    protected_bytes(evidence_source / 'main.go', CHAIN_EVIDENCE.encode())
    paxd = shutil.which('paxd')
    require(paxd is not None, 'native paxd prerequisite executable missing')
    paxd = Path(paxd).resolve(strict=True)
    commands = [
        ['cargo', 'build', '--locked', '--manifest-path', 'services/agent/Cargo.toml',
         '-p', 'layerx-client', '--example', 'inference_room_writes'],
        ['cargo', 'build', '--locked', '--manifest-path', 'services/platform/Cargo.toml',
         '-p', 'layerx-indexer', '--bin', 'layerx-indexer', '--bin', 'layerx-ai-source'],
        ['cargo', 'build', '--locked', '--manifest-path', 'services/platform/Cargo.toml',
         '-p', 'layerx-platform-gateway', '--bin', 'layerx-gateway'],
        ['cargo', 'build', '--locked', '--manifest-path', 'services/platform/Cargo.toml',
         '-p', 'layerx-runtime-clock', '--bin', 'layerx-runtime-clock'],
        ['cargo', 'build', '--locked', '--manifest-path', 'services/platform/Cargo.toml',
         '-p', 'layerx-platform-authority', '--bin', 'layerx-receipt-authority'],
        ['go', '-C', str(ROOT / 'services/wallet/attestor'), 'build', '-mod=readonly',
         '-o', support['attestor'], './cmd/attestor'],
        ['go', '-C', str(share_source), 'build', '-mod=mod', '-o', support['custody-shares'], '.'],
        ['go', '-C', str(evidence_source), 'build', '-mod=mod', '-o', support['chain-evidence'], '.'],
        ['node', '--check', str(native)],
        ['node', str(native), 'model', str(directory / 'model')],
    ]
    with (directory / 'build.log').open('xb') as log:
        for command in commands:
            result = subprocess.run(command, cwd=ROOT, stdout=log, stderr=log, timeout=1800)
            require(result.returncode == 0, 'room producer build failed; inspect ' + str(directory / 'build.log'))
    for index, name in enumerate(name for name in OWN if name.endswith('.py')):
        py_compile.compile(str(ROOT / name), cfile=str(directory / (str(index) + '.pyc')), doraise=True)
    artifacts = [agent, encoder, paxd, *map(Path, support.values()), ROOT / 'build/liblayerx.a', native,
                 ROOT / 'programs/crates/layerx-programs-ai-market/tests/fixtures/ai-market-opt-z.hex',
                 *sorted((directory / 'model').iterdir())]
    require(start['source_digest'] == fingerprint(), 'room sources changed during artifact production')
    save(directory / 'manifest.json', {**start, 'commands': commands,
                                      'artifacts': [envelope.record(path) for path in artifacts],
                                      'agent': str(agent), 'native': str(native), 'encoder': str(encoder),
                                      'support': support,
                                      'paxd': {**envelope.record(paxd),
                                               'source_revision': start['source_revision']}})
    print('INFERENCE_ROOM_ARTIFACTS ' + str(directory / 'manifest.json'), flush=True)


def built_artifacts():
    directory = active_build()
    manifest = load(directory / 'manifest.json', 'room artifact manifest')
    require(manifest['schema'] == SCHEMA and manifest['task'] == TASK
            and manifest['builder'] == os.environ['PAXEER_X_BUILDER_ID']
            and manifest['source_digest'] == fingerprint(), 'room build source or builder mismatch')
    for row in manifest['artifacts']:
        require(envelope.record(row['path']) == row, 'room artifact changed: ' + row['path'])
    return directory, manifest


def hex32(value, name):
    require(isinstance(value, str) and re.fullmatch('[0-9a-f]{64}', value)
            and value != '0' * 64, name + ' must be nonzero 32-byte lowercase hex')
    return value


def encoded(manifest, operation, value):
    result = subprocess.run([manifest['encoder'], operation], input=json.dumps(value),
                            capture_output=True, text=True, cwd=ROOT, timeout=30)
    require(result.returncode == 0,
            'product encoder ' + operation + ' refused: ' + result.stderr.strip())
    return json.loads(result.stdout)


def protected_bytes(path, value):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, 'wb') as stream:
        stream.write(value)
        stream.flush()
        os.fsync(stream.fileno())


class RoomCustody:
    """Provision the run's keys in five real attestors using product share codecs."""

    def __init__(self, driver):
        self.driver = driver
        self.directory = driver.directory / 'custody'
        self.directory.mkdir(mode=0o700)
        self.ids = ['room-node-' + str(index) for index in range(5)]
        self.issuer = 'inference-room-' + uuid.uuid4().hex
        self.authority = ec.generate_private_key(ec.SECP256R1())
        protected_bytes(self.directory / 'authority.key', self.authority.private_bytes(
            serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, NoEncryption()))
        protected_bytes(self.directory / 'authority.pem', self.authority.public_key().public_bytes(
            serialization.Encoding.PEM, serialization.PublicFormat.SubjectPublicKeyInfo))
        self.records = []
        self.shares = {}
        for name in ('host', 'evidence0', 'evidence1', 'evidence2'):
            account = driver.config['accounts'][name]
            result = subprocess.run([driver.manifest['support']['custody-shares']],
                                    input=json.dumps({'Seed': Path(account['delegate_seed']).read_bytes().hex(),
                                                      'Secret': Path(account['secret']).read_text(),
                                                      'Members': self.ids}),
                                    capture_output=True, text=True, timeout=30)
            require(result.returncode == 0, 'product custody share preparation failed: ' + result.stderr)
            shares = json.loads(result.stdout)
            require(shares['ed25519']['public_key'] == account['peer_key']
                    and '0x' + keccak256(bytes.fromhex(shares['secp256k1']['public_key'])[1:])[-20:].hex()
                    == account['address'], 'custody shares must retain the run identity')
            self.shares[name] = shares
            for curve in ('ed25519', 'secp256k1'):
                self.records.append({'key_id': name + '-' + curve, 'epoch': 0, 'curve': curve,
                                     'public_key': shares[curve]['public_key'], 'owner': name,
                                     'operations': ['import', 'refresh', 'sign']})
        ca = CaMaterial(self.directory)
        self.ca = ca
        operator_directory = self.directory / 'operator'
        operator_directory.mkdir(mode=0o700)
        operator_ca = CaMaterial(operator_directory)
        operator_cert, operator_key = operator_ca.issue('room-key-operator', [], True)
        self.operator_context = ssl.create_default_context(cafile=str(ca.ca_pem))
        self.operator_context.load_cert_chain(str(operator_cert), str(operator_key))
        self.ports = reserve_ports(11)
        driver.custody_ports = self.ports
        api = [sock.getsockname()[1] for sock in self.ports[:5]]
        peer = [sock.getsockname()[1] for sock in self.ports[5:10]]
        self.addresses = ['127.0.0.1:' + str(port) for port in api]
        pins, leaves = [], []
        with (self.directory / 'tls.log').open('xb') as log:
            def openssl(*args):
                result = subprocess.run(['openssl', *map(str, args)], stdout=log, stderr=log, timeout=30)
                require(result.returncode == 0, 'custody TLS failed; inspect custody/tls.log')
            for name in self.ids:
                cert, key = ca.issue(name, [], False, ['localhost'])
                extensions = cert.parent / 'custody.ext'
                protected_bytes(extensions, b'basicConstraints=CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=serverAuth,clientAuth\nsubjectAltName=IP:127.0.0.1,DNS:localhost\n')
                openssl('x509', '-req', '-in', cert.parent / 'leaf.csr', '-CA', ca.ca_pem,
                        '-CAkey', ca._key, '-set_serial', str(2000 + len(pins)), '-days', '1',
                        '-extfile', extensions, '-out', cert)
                public = x509.load_pem_x509_certificate(cert.read_bytes()).public_key()
                pins.append(hashlib.sha256(public.public_bytes(
                    serialization.Encoding.DER, serialization.PublicFormat.SubjectPublicKeyInfo)).hexdigest())
                leaves.append((cert, key))
            client, private = ca.issue('room-daemon', [], True)
            client_der, private_der = self.directory / 'client.der', self.directory / 'client.pk8'
            openssl('x509', '-in', client, '-outform', 'DER', '-out', client_der)
            openssl('pkcs8', '-topk8', '-nocrypt', '-in', private, '-outform', 'DER', '-out', private_der)
        self.context = ssl.create_default_context(cafile=str(ca.ca_pem))
        self.context.load_cert_chain(str(client), str(private))
        self.members = [{'id': name, 'spki_sha256': pin} for name, pin in zip(self.ids, pins)]
        self.sequence = 0
        self.inventory()
        jwks_port = self.ports[10].getsockname()[1]
        public = self.authority.public_key().public_numbers()
        self.b64 = lambda data: base64.urlsafe_b64encode(data).decode().rstrip('=')
        jwks = self.directory / 'jwks.json'
        save(jwks, {'keys': [{'kty': 'EC', 'crv': 'P-256', 'use': 'sig', 'alg': 'ES256',
                              'kid': 'room-identity', 'x': self.b64(public.x.to_bytes(32, 'big')),
                              'y': self.b64(public.y.to_bytes(32, 'big'))}]})
        issuer_config = self.directory / 'issuer.json'
        save(issuer_config, {'port': jwks_port, 'jwks': str(jwks)})
        self.ports[10].close()
        driver.processes.start('identity-issuer', [sys.executable, str(Path(__file__).resolve()),
                                                  '--identity-issuer', issuer_config], driver.runtime.env)
        driver.processes.wait(lambda: http_json('http://127.0.0.1:' + str(jwks_port))[0] == 200)
        policy = self.directory / 'policy.json'
        save(policy, {'version': 1, 'defaults': {'chain_id': 125, 'kinds': ['evm_tx'],
                    'caps': {'native': {'per_transaction': '1000000000000000000',
                                        'daily': '100000000000000000000'}},
                    'rate_per_minute': 1000, 'destinations_allow': [PROGRAMS, PAY],
                    'program_calls': [{'program_id': driver.config['program'],
                                       'entrypoints': ['layerx_call'],
                                       'fee_limit': str(driver.config['execution_fee_limit'])}],
                    'selectors': {
                        PROGRAMS: ['0x' + keccak256(b'call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)')[:4].hex()],
                        PAY: ['0x' + keccak256(b'streamMeter(bytes32,uint64,bytes32,bytes)')[:4].hex()]}}})
        for index, name in enumerate(self.ids):
            store_key = self.directory / (name + '.store-key')
            protected_bytes(store_key, os.urandom(32).hex().encode())
            environment = dict(driver.runtime.env, ATTESTOR_NODE_ID=name, ATTESTOR_REGION='room',
                ATTESTOR_NODE_KEY_FILE=str(store_key), ATTESTOR_DATA_DIR=str(self.directory / name),
                ATTESTOR_LISTEN_ADDR=self.addresses[index],
                ATTESTOR_PEER_LISTEN_ADDR='127.0.0.1:' + str(peer[index]),
                ATTESTOR_PEERS=','.join(other + '=127.0.0.1:' + str(peer[j])
                                       for j, other in enumerate(self.ids) if j != index),
                ATTESTOR_PEER_PINS=','.join(other + '=' + pins[j]
                                           for j, other in enumerate(self.ids) if j != index),
                ATTESTOR_CHAIN_ID='125', ATTESTOR_CEREMONY='true',
                ATTESTOR_TLS_CERT_FILE=str(leaves[index][0]), ATTESTOR_TLS_KEY_FILE=str(leaves[index][1]),
                ATTESTOR_TLS_CA_FILE=str(ca.ca_pem), ATTESTOR_OPERATOR_CA_FILE=str(operator_ca.ca_pem),
                ATTESTOR_INVENTORY_FILE=str(self.directory / 'inventory.jwt'),
                ATTESTOR_INVENTORY_PUBLIC_KEY_FILE=str(self.directory / 'authority.pem'),
                ATTESTOR_AUTHORITY_PUBLIC_KEY_FILE=str(self.directory / 'authority.pem'),
                ATTESTOR_AUTHORITY_ISSUER=self.issuer, ATTESTOR_AUTHORITY_TENANT='inference-room',
                ATTESTOR_JWKS_URL='http://127.0.0.1:' + str(jwks_port),
                ATTESTOR_JWT_ISSUER=self.issuer, ATTESTOR_JWT_AUDIENCE='room-daemon',
                ATTESTOR_POLICY_FILE=str(policy))
            self.ports[index].close()
            self.ports[5 + index].close()
            driver.processes.start(name, [driver.manifest['support']['attestor']], environment)
        for index in range(5):
            driver.processes.wait(lambda index=index: self.healthy(index))
        for name, shares in self.shares.items():
            account = driver.config['accounts'][name]
            for curve, value in shares.items():
                request = {'ceremony_id': self.issuer, 'epoch': 0, 'public_key': value['public_key'],
                           'participants': self.ids, 'threshold': 3, 'session_id': 'import-' + name + '-' + curve,
                           'key_id': name + '-' + curve, 'owner': name, 'account': account['address']}
                self.all('/v1/keys/import', [{**request, 'share': value['shares'][node]} for node in self.ids])
                if curve == 'secp256k1':
                    refresh = {'session_id': 'refresh-' + name, 'key_id': request['key_id'],
                               'ceremony_id': self.issuer, 'import_session_id': request['session_id'],
                               'expected_epoch': 0}
                    self.all('/v1/keys/refresh', [refresh] * 5)
                    next(record for record in self.records if record['key_id'] == request['key_id'])['epoch'] = 1
                    self.inventory()
        self.signers = {}
        for name in self.shares:
            account = driver.config['accounts'][name]
            assertion = self.directory / (name + '.jwt')
            now = int(time.time())
            protected_bytes(assertion, self.jwt({'iss': self.issuer, 'aud': 'room-daemon', 'sub': name,
                                                  'iat': now, 'exp': now + 1800,
                                                  'jti': uuid.uuid4().hex}, 'JWT', 'room-identity').encode())
            self.signers[name] = {'signer': {'nodes': [{'id': node, 'address': address}
                                                      for node, address in zip(self.ids, self.addresses)],
                                    'trust_anchors': [str(ca.ca_der)], 'client_certificate_chain': [str(client_der)],
                                    'client_private_key_pkcs8': str(private_der),
                                    'key_id': name + '-ed25519', 'public_key': list(bytes.fromhex(account['peer_key'])),
                                    'evm_key_id': name + '-secp256k1',
                                    'evm_address': list(bytes.fromhex(account['address'][2:])),
                                    'signers': self.ids[:3], 'network_id': 125, 'timeout_seconds': 30},
                                  'assertion_source': str(assertion)}
        save(self.directory / 'signers.json', self.signers)

    def jwt(self, claims, kind, kid=None):
        b64 = lambda data: base64.urlsafe_b64encode(data).decode().rstrip('=')
        header = {'alg': 'ES256', 'typ': kind}
        if kid is not None:
            header['kid'] = kid
        message = '.'.join(b64(json.dumps(value, separators=(',', ':')).encode())
                           for value in (header, claims))
        r, s = utils.decode_dss_signature(self.authority.sign(message.encode(), ec.ECDSA(hashes.SHA256())))
        return message + '.' + b64(r.to_bytes(32, 'big') + s.to_bytes(32, 'big'))

    def inventory(self):
        self.sequence += 1
        now = int(time.time())
        token = self.jwt({'version': 1, 'iss': self.issuer, 'aud': 'wallet-custody-inventory',
                           'tenant': 'inference-room', 'sequence': str(self.sequence),
                           'iat': now, 'exp': now + 1800, 'protocol': 'wallet', 'threshold': 3,
                           'members': self.members, 'keys': self.records}, 'wallet-custody-inventory+jwt')
        pending = self.directory / ('inventory-' + str(self.sequence) + '.jwt')
        protected_bytes(pending, token.encode())
        pending.replace(self.directory / 'inventory.jwt')

    def healthy(self, index):
        connection = http.client.HTTPSConnection('127.0.0.1', int(self.addresses[index].split(':')[1]),
                                                  timeout=60, context=self.context)
        try:
            connection.request('GET', '/health')
            response = connection.getresponse()
            raw = response.read(1_048_577)
            require(len(raw) <= 1_048_576, 'attestor health response exceeds bound')
            report = json.loads(raw)
            require(report['node_id'] == self.ids[index] and type(report['ready']) is bool,
                    'attestor health identity or readiness differs')
            require(response.status == (200 if report['ready'] else 503),
                    'attestor health status differs from readiness')
            return report['ready']
        finally:
            connection.close()

    def request(self, index, route, body=None):
        connection = http.client.HTTPSConnection('127.0.0.1', int(self.addresses[index].split(':')[1]),
                                                  timeout=60, context=self.operator_context if route in
                                                  ('/v1/keys/import', '/v1/keys/refresh') else self.context)
        try:
            connection.request('GET' if body is None else 'POST', route,
                               None if body is None else json.dumps(body), {'Content-Type': 'application/json'})
            response = connection.getresponse()
            raw = response.read(1_048_577)
            require(len(raw) <= 1_048_576 and response.status == 200,
                    'real attestor refused ' + route + ': ' + str(response.status) + ' ' + raw[:4096].decode())
            return json.loads(raw)
        finally:
            connection.close()

    def all(self, route, bodies):
        with ThreadPoolExecutor(max_workers=5) as executor:
            futures = [executor.submit(self.request, index, route, body) for index, body in enumerate(bodies)]
            results = [future.result() for future in futures]
        for index, result in enumerate(results):
            require(result['node_id'] == self.ids[index], 'attestor node identity differs')
        save(self.directory / ('operation-' + uuid.uuid4().hex + '.json'), {'route': route, 'results': results})
        return results


def configuration(directory, build_directory, manifest):
    directory = Path(directory)
    keys = directory / 'keys'
    keys.mkdir(mode=0o700)
    generated = {}
    for name in ('host', 'asker', 'peer0', 'peer1', 'evidence0', 'evidence1', 'evidence2'):
        seed = os.urandom(64)
        derived = accounts.derive_from_seed(seed)
        require(derived.evm_private_key is not None, 'generated native signer key')
        protected_bytes(keys / (name + '.seed'), seed)
        protected_bytes(keys / (name + '.secp256k1'), derived.evm_private_key.hex().encode())
        protected_bytes(keys / (name + '.ed25519'), derived.layerx_seed)
        key = envelope.Ed25519PrivateKey.from_private_bytes(derived.layerx_seed)
        identity = keys / (name + '.pkcs8')
        protected_bytes(identity, key.private_bytes(envelope.Encoding.DER, PrivateFormat.PKCS8,
                                                   NoEncryption()))
        identifiers = encoded(manifest, 'Identity', {'did': derived.did,
                                                     'public_key': derived.layerx_public_key})
        generated[name] = {**identifiers, 'address': derived.evm_address.lower(),
                           'did': derived.did, 'peer_key': derived.layerx_public_key,
                           'secret': str(keys / (name + '.secp256k1')),
                           'delegate_seed': str(keys / (name + '.ed25519')),
                           'identity': str(identity)}
    meter_seed = os.urandom(32)
    protected_bytes(keys / 'host-meter.ed25519', meter_seed)
    meter = envelope.Ed25519PrivateKey.from_private_bytes(meter_seed)
    reference = load(build_directory / 'model/reference.json', 'scripted reference')
    profile = {'minimum_bond': '1000', 'canary_ratio_bps': 0, 'lap_size_tokens': 5,
               'challenge_window_blocks': 64, 'slash_fraction_bps': 1000,
               'host_share_bps': 1000, 'protocol_share_bps': 100,
               'settlement_account': generated['host']['main_account'],
               'max_generated_tokens': 4, 'queue_depth': 1,
               'liveness_timeout_ms': 120000, 'acknowledgement_window_ms': 60000,
               'canary_relative_tolerance_bps': 0}
    room = {'room_id': os.urandom(32).hex(),
            'host_peer_key': generated['host']['principal'],
            'runtime_digest': hashlib.sha256(b'paxeer-inference-room-scripted-cpu-v1').hexdigest(),
            'profile': profile}
    peers = [{**generated['peer' + str(index)], 'owner': generated['peer' + str(index)]['principal'],
              'manifest': str(directory / ('peer' + str(index) + '-manifest.bin')),
              'nonce': os.urandom(32).hex(), 'bid_id': os.urandom(32).hex()}
             for index in range(2)]
    config = {'schema': SCHEMA, 'accounts': generated, 'room': room, 'peers': peers,
              'evaluator_nonce': os.urandom(32).hex(),
              'program': os.urandom(32).hex(), 'ask_id': os.urandom(32).hex(), 'price': '1000',
              'model_digest': reference['model_digest'], 'commitment_digest': os.urandom(32).hex(),
              'host_meter_key': meter.public_key().public_bytes(envelope.Encoding.Raw,
                                                               envelope.PublicFormat.Raw).hex(),
              'writes': {phase: [] for phase in WRITE_PHASES},
              'room_offer': {'room_id': list(bytes.fromhex(room['room_id'])),
                             'model_digest': list(bytes.fromhex(reference['model_digest'])),
                             'runtime_digest': list(bytes.fromhex(room['runtime_digest'])),
                             'layer_plan': [{'peer_key': list(bytes.fromhex(peer['peer_key'])),
                                             'first_layer': index * 2, 'end_layer': (index + 1) * 2}
                                            for index, peer in enumerate(peers)], 'price': '1000'}}
    save(directory / 'configuration.json', config)
    save(directory / 'write-ownership.json', WRITE_OWNERS)
    return config


def substituted(value, values):
    if isinstance(value, dict):
        return {name: substituted(item, values) for name, item in value.items()}
    if isinstance(value, list):
        return [substituted(item, values) for item in value]
    return string.Template(value).substitute(values) if isinstance(value, str) else value


def http_json(url, method='GET', body=None, bearer=None):
    parsed = urlsplit(url)
    require(not parsed.query and not parsed.fragment, 'room HTTP route must not carry query or fragment')
    origin = RootRpc(parsed.scheme + '://' + parsed.netloc)
    headers = {'Content-Type': 'application/json'}
    if bearer:
        headers['Authorization'] = 'Bearer ' + bearer
    connection = http.client.HTTPConnection(origin.url.hostname, origin.url.port, timeout=60)
    try:
        connection.request(method, parsed.path or '/', None if body is None else json.dumps(body), headers)
        response = connection.getresponse()
        raw = response.read(1_048_577)
        require(len(raw) <= 1_048_576 and response.getheader('Content-Type', '').split(';')[0] == 'application/json',
                'bounded room HTTP JSON response required')
        return response.status, json.loads(raw)
    finally:
        connection.close()


def writes(runtime, config, phases):
    receipts = []
    for phase in phases:
        for index, row in enumerate(config['writes'][phase]):
            validate_write(row, phase)
            receipt = runtime.recorded_submit(phase + '-' + str(index), row['raw'])
            require(receipt['transactionHash'] == '0x' + keccak256(bytes.fromhex(row['raw'][2:])).hex(),
                    'native chain write reference differs from signed bytes')
            receipts.append(receipt)
    return receipts


def validate_write(row, name):
    require(isinstance(row['raw'], str) and re.fullmatch('0x([0-9a-f]{2})+', row['raw']),
            'canonical signed native chain write required: ' + name)
    transaction, _ = decode_signed(bytes.fromhex(row['raw'][2:]))
    require(transaction.chain_id == 125 and '0x' + transaction.to.hex() == row['to']
            and hashlib.sha256(transaction.data).hexdigest() == row['calldata_sha256'],
            'signed chain write does not bind its declared calldata: ' + name)


class GeneratedRoomWrites:
    """Prepare native writes with product codecs and the run's protected keys."""

    def __init__(self, driver):
        self.driver = driver
        self.runtime = driver.runtime
        self.config = driver.config

    def view(self, target, name, kinds, arguments, outputs):
        data = encode_call(name, kinds, arguments)
        raw = self.runtime.rpc.request('eth_call', [{'to': target, 'data': '0x' + data.hex()}, 'latest'])
        return decode_abi(outputs, bytes.fromhex(raw.removeprefix('0x')))

    def signed(self, account, target, data, value=0):
        require(self.runtime.rpc.request('eth_chainId', []) == '0x7d', 'sign only for the owned chain 125')
        nonce = int(self.runtime.rpc.request('eth_getTransactionCount', [account['address'], 'latest']), 16)
        price = int(self.runtime.rpc.request('eth_gasPrice', []), 16)
        transaction = Type2Transaction(125, nonce, 0, price * 2, 30_000_000,
                                       bytes.fromhex(target.removeprefix('0x')), value, data)
        secret = int(Path(account['secret']).read_text().strip(), 16)
        raw = sign_transaction(secret, transaction)
        row = {'raw': '0x' + raw.hex(), 'to': target,
               'calldata_sha256': hashlib.sha256(data).hexdigest()}
        validate_write(row, 'generated')
        decoded, signature = decode_signed(raw)
        require(decoded.recover_signer(signature).lower() == account['address'],
                'generated signed write belongs to its funded account')
        return row

    def submit(self, name, account, target, data, phase=None, value=0):
        row = self.signed(account, target, data, value)
        save(self.driver.directory / ('prepared-' + name + '.json'), row)
        if phase is not None:
            self.config['writes'][phase].append(row)
        receipt = self.runtime.recorded_submit(name, row['raw'])
        self.driver.last_write_height = int(receipt['blockNumber'], 16)
        require(receipt['transactionHash'] == '0x' + keccak256(bytes.fromhex(row['raw'][2:])).hex(),
                'native receipt differs from the generated signed write')
        return receipt

    def next_committed_height(self):
        previous = self.driver.last_write_height
        target = previous + 2
        if int(self.runtime.rpc.request('eth_blockNumber', []), 16) < target:
            host = self.config['accounts']['host']
            self.submit('advance-height-' + uuid.uuid4().hex[:12], host,
                        host['address'], b'')
        self.runtime.wait(lambda: int(self.runtime.rpc.request('eth_blockNumber', []), 16)
                          >= target)
        return int(self.runtime.rpc.request('eth_blockNumber', []), 16) - 1

    def bind_accounts(self):
        for name, account in self.config['accounts'].items():
            state = accounts.read_layerx_bind_state(self.runtime.rpc.request, account['address'])
            seed = Path(account['delegate_seed']).read_bytes()
            secret = bytes.fromhex(Path(account['secret']).read_text())
            derived = accounts.DerivedAccount(0, account['address'], account['peer_key'],
                                               account['did'], seed, secret)
            plan = accounts.plan_layerx_bind(derived, 125, state)
            require(plan.action == 'bind' and plan.call is not None,
                    'fresh generated native account must bind exactly once')
            self.submit('bind-' + name, account, plan.call.to, bytes.fromhex(plan.call.data[2:]))
            bound = accounts.read_layerx_bind_state(self.runtime.rpc.request, account['address'])
            require(bound.bound_did_public_key == account['peer_key'] and bound.nonce == state.nonce + 1,
                    'native chain did not retain the generated binding')

    def deploy(self):
        version, *coefficients = self.view(PROGRAMS, 'getCurrentMeteringSchedule', [], [],
                                           ['uint32', *(['uint64'] * 9)])
        require(version > 0 and all(value > 0 for value in coefficients),
                'native program deployment requires its governed metering schedule')
        save(self.driver.directory / 'deployment-metering.json',
             {'version': version, 'coefficients': coefficients})
        wasm_path = ROOT / 'programs/crates/layerx-programs-ai-market/tests/fixtures/ai-market-opt-z.hex'
        wasm = bytes.fromhex(wasm_path.read_text())
        account = self.config['accounts']['host']
        deployment = NativeProgramDeploy(bytes.fromhex(self.config['program']), 2,
                                         bytes.fromhex(account['principal']),
                                         hashlib.sha256(ABI2_GUEST).digest(), ABI2_GUEST, policy=1)
        protected_bytes(self.driver.directory / 'deploy-payload.bin', deployment.encode())
        data = encode_call('deploy', ['bytes32', 'uint16', 'uint8', 'bytes32', 'bytes32', 'bytes', 'bytes'],
                           [deployment.program_id, deployment.guest_abi, deployment.policy,
                            deployment.authority, deployment.new_hash, b'', ABI2_GUEST])
        deploy_receipt = self.submit('deploy-program', account, PROGRAMS, data, 'deploy')
        self.deployment_admission(deploy_receipt, ABI2_GUEST)
        *_, asset = self.view(PROGRAMS, 'getCurrentFeeSchedule', [], [],
                             ['uint32', *(['uint64'] * 7), 'bytes32'])
        self.config['asset'] = asset.hex()
        market = encoded(self.driver.manifest, 'CreateMarket', {
            'program': self.config['program'], 'owner': account['principal'],
            'asset': asset.hex(), 'refund': account['main_account'],
            'model_digest': self.config['model_digest'],
            'commitment_digest': self.config['commitment_digest'],
            'epoch_budget_cap': '1000000', 'minimum_epoch_funding': '1'})
        registration = encode_call('registerAccount', ['bytes32', 'uint8', 'bytes32', 'bytes'],
                                   [deployment.program_id, 2, asset, b'paxai/rewards/v1'])
        registration_receipt = self.submit('register-rewards-abi2', account, PROGRAMS,
                                           registration, 'deploy')
        registered, registered_asset, sequence = self.view(
            PROGRAMS, 'getProgramAccount', ['bytes32', 'bytes32', 'bytes'],
            [deployment.program_id, asset, b'paxai/rewards/v1'],
            ['bytes32', 'bytes32', 'uint64'])
        require(registered.hex() == market['rewards'] and registered_asset == asset and sequence > 0,
                'ABI 2 registration must bind the AI market rewards account and fee asset')
        upgrade = NativeProgramUpgrade(deployment.program_id, 5, deployment.new_hash,
                                       hashlib.sha256(wasm).digest(), wasm)
        protected_bytes(self.driver.directory / 'upgrade-payload.bin', upgrade.encode())
        upgrade_data = encode_call('upgrade',
                                   ['bytes32', 'uint16', 'uint8', 'bytes32', 'bytes32',
                                    'bytes', 'bytes', 'bytes', 'uint128'],
                                   [upgrade.program_id, upgrade.guest_abi, 0, upgrade.old_hash,
                                    upgrade.new_hash, b'', b'', wasm, 0])
        upgrade_receipt = self.submit('upgrade-program-abi5', account, PROGRAMS,
                                      upgrade_data, 'deploy')
        self.deployment_admission(upgrade_receipt, wasm)
        policy, authority, code_hash, abi, version, active = self.view(
            PROGRAMS, 'getProgram', ['bytes32'], [deployment.program_id],
            ['uint8', 'bytes32', 'bytes32', 'uint16', 'uint32', 'bool'])
        require((policy, authority, code_hash, abi, version, active)
                == (deployment.policy, deployment.authority, upgrade.new_hash, 5, 2, True),
                'upgraded native program must retain the AI market code, authority and ABI 5')
        program_read = {'policy': policy, 'authority': authority.hex(),
                        'code_hash': code_hash.hex(), 'abi': abi, 'version': version,
                        'active': active}
        save(self.driver.directory / 'program-after-upgrade.json', program_read)
        save(self.driver.directory / 'program-deployment.json',
             {'program': self.config['program'], 'wasm': envelope.record(wasm_path),
              'code_hash': upgrade.new_hash.hex(),
              'deploy': {'receipt': deploy_receipt, 'transaction':
                         load(self.driver.directory / 'prepared-deploy-program.json', 'ABI 2 deploy')},
              'registration': {'receipt': registration_receipt, 'transaction':
                               load(self.driver.directory / 'prepared-register-rewards-abi2.json',
                                    'ABI 2 account registration')},
              'upgrade': {'receipt': upgrade_receipt, 'transaction':
                          load(self.driver.directory / 'prepared-upgrade-program-abi5.json',
                               'ABI 5 upgrade')},
              'get_program': program_read})

    def operation(self, name, parameters, account, state=None, phase=None, delegate=False):
        version, ceiling, asset = self.view(PROGRAMS, 'quoteCallFee', ['uint64[7]'],
                                            [RESOURCES], ['uint32', 'uint128', 'bytes32'])
        require(version > 0 and asset.hex() == self.config['asset'],
                'program call fee quote must use the registered market asset')
        self.config['fee_limit'] = ceiling + 1_000_000
        self.config['execution_fee_limit'] = self.config['fee_limit']
        occupancy = None
        if account['principal'] == self.config['accounts']['host']['principal']:
            *prices, schedule_asset = self.view(PROGRAMS, 'getFeeSchedule', ['uint32'],
                                                [version], [*(['uint64'] * 7), 'bytes32'])
            require(schedule_asset == asset and prices[6] > 0,
                    'owner occupancy price must match the quoted fee schedule')
            self.config['fee_limit'] = ceiling + OWNER_NAMESPACE_BYTES * OWNER_RENT_BATCHES * prices[6]
            self.config['owner_fee_limit'] = self.config['fee_limit']
            fee_account, = self.view(LEDGER, 'accountId', ['bytes32', 'bytes32'],
                                     [bytes.fromhex(account['peer_key']), asset], ['bytes32'])
            balance, = self.view(LEDGER, 'balanceOf', ['bytes32', 'bytes32'],
                                 [fee_account, asset], ['uint128'])
            require(balance >= self.config['fee_limit'],
                    'host fee account must cover the signed occupancy ceiling')
            occupancy = {'byte_batch_price': prices[6], 'namespace_byte_limit': OWNER_NAMESPACE_BYTES, 'prepaid_batches': OWNER_RENT_BATCHES,
                         'fee_account': fee_account.hex(), 'fee_balance': str(balance)}
        height = int(self.runtime.rpc.request('eth_blockNumber', []), 16)
        request_id = os.urandom(32).hex()
        inputs = {**parameters, 'height': height, 'request': request_id}
        if state is not None:
            inputs['state'] = state
        payload = encoded(self.driver.manifest, name, inputs)['payload']
        save(self.driver.directory / ('program-call-fee-quote-' + request_id[:12] + '.json'),
             {'operation': name, 'height': height, 'version': version, 'ceiling': ceiling,
              'asset': asset.hex(), 'signed_limit': self.config['fee_limit'],
              'owner_occupancy': occupancy})
        request = {'operation': 'CREATE' if name == 'CreateMarket' else name, 'payload': payload,
                   'program': self.config['program'], 'actor': account['principal'],
                   'height': height + 1, 'expiry': height + 256, 'request': request_id}
        if state is not None:
            request['state'] = state
        if delegate:
            request['delegate_seed'] = Path(account['delegate_seed']).read_bytes().hex()
        envelope_bytes = encoded(self.driver.manifest, 'Envelope', request)['payload']
        call_terms = {'operation': request['operation'], 'program': self.config['program'],
                      'actor': account['principal'], 'payload': envelope_bytes,
                      'resources': RESOURCES, 'fee_limit': str(self.config['fee_limit'])}
        if name in ('LockBond', 'FUND'):
            call_terms.update(asset=self.config['asset'], rewards=self.config['rewards'],
                              amount=parameters['amount'])
        call = encoded(self.driver.manifest, 'NativeCall', call_terms)
        try:
            return self.submit(name.lower() + '-' + request['request'][:12], account,
                               '0x' + call['to'], bytes.fromhex(call['payload']), phase)
        except RuntimeError:
            native = decode_native_program_call(bytes.fromhex(call['native_call']))
            simulation = encode_call('simulateCall',
                                     ['bytes32', 'uint16', 'string', 'bytes', 'bytes', 'bytes',
                                      'uint32', 'uint64[7]', 'uint128'],
                                     [native.program_id, native.guest_abi, native.entrypoint,
                                      native.calldata, native.capabilities, native.access_declaration,
                                      native.response_capacity, native.resources,
                                      self.config['fee_limit']])
            diagnostic = {'operation': name, 'height': height}
            if state is not None:
                diagnostic['application'] = encoded(self.driver.manifest, 'TransitionResult', {
                    'state': state, 'envelope': native.calldata.hex(),
                    'actor': account['principal'], 'height': height + 1})
            try:
                diagnostic['result'] = self.runtime.rpc.request('eth_call', [
                    {'from': account['address'], 'to': '0x' + call['to'],
                     'data': '0x' + simulation.hex()}, 'latest'])
            except (OSError, RuntimeError, ValueError, http.client.HTTPException) as error:
                diagnostic['error'] = str(error)
            save(self.driver.directory / ('program-simulation-' + request_id[:12] + '.json'), diagnostic)
            raise

    def deployment_admission(self, receipt, wasm):
        height = int(receipt['blockNumber'], 16)
        endpoint = self.runtime.config['consensus_rpc_url']
        def consensus(method, params):
            return RootRpc(endpoint).request(method, params)
        self.driver.processes.wait(lambda: int(consensus('status', {})['sync_info']['latest_block_height']) > height + 1)
        block = consensus('block', {'height': str(height)})
        results = consensus('block_results', {'height': str(height)})
        validators = consensus('validators', {'height': str(height), 'per_page': '100'})
        require(int(validators['total']) == len(validators['validators']), 'complete deployment validator set required')
        def material(at, execution=False):
            signed = consensus('commit', {'height': str(at)})
            require(signed['canonical'] is True, 'canonical deployment commit required')
            request = {'signed': signed['signed_header'], 'validators': validators['validators']}
            if execution:
                request.update(transactions=block['block']['data']['txs'],
                               results=results['txs_results'], transaction_hash=receipt['transactionHash'])
            result = subprocess.run([self.driver.manifest['support']['chain-evidence']],
                                    input=json.dumps(request), capture_output=True, text=True,
                                    cwd=ROOT, timeout=15)
            require(result.returncode == 0, 'chain deployment evidence encoding refused: ' + result.stderr)
            return json.loads(result.stdout)
        previous, current = material(height), material(height + 1, True)
        def witness(head, key, at):
            for retry in range(9):
                response = consensus('abci_query', {'path': '/store/lxphost/key', 'data': key.hex(),
                                                     'height': str(at), 'prove': True})['response']
                if (response['code'] != 36 or not response.get('log', '').startswith('historical proof rate limited')
                        or retry == 8):
                    break
                time.sleep(.25)
            require(response['code'] == 0 and int(response['height']) == at
                    and base64.b64decode(response['key'], validate=True) == key,
                    'deployment store proof refused: ' + str(response))
            return {**{field: head[field] for field in ('signed_header', 'commit', 'validator_set')},
                    'key': key.hex(), 'value': None if not response.get('value') else
                    base64.b64decode(response['value'], validate=True).hex(),
                    'ops': [{'type': op['type'], 'key': base64.b64decode(op['key'], validate=True).hex(),
                             'data': base64.b64decode(op['data'], validate=True).hex()}
                            for op in response['proofOps']['ops']]}
        program = bytes.fromhex(self.config['program'])
        prefix = b'\x01\x00\x09'
        request = {'program': self.config['program'], 'code': wasm.hex(),
                   'previous': witness(previous, prefix + b'program\0' + program, height - 1),
                   'current': witness(current, prefix + b'program\0' + program, height),
                   'interface': witness(current, prefix + b'interface\0' + program, height),
                   'status': witness(current, prefix + b'wind-down\0s' + program, height),
                   'now_ms': int(time.time() * 1000), 'execution': current,
                   'validators': [{'public_key': bytes(row['pub_key']).hex(), 'voting_power': row['voting_power']}
                                  for row in self.runtime.manifest['validators']],
                   'chain_id': self.runtime.manifest['chain_id']}
        save(self.driver.directory / ('deployment-material-' + str(height) + '.json'), request)
        admission = encoded(self.driver.manifest, 'DeploymentAdmission', request)
        journal = self.driver.directory / 'deployment-journal'
        journal.mkdir(mode=0o700, exist_ok=True)
        for suffix in ('admission', 'deployment'):
            protected_bytes(journal / (admission['digest'] + '.' + suffix), bytes.fromhex(admission[suffix]))

    def snapshot(self):
        required_sequence = self.driver.last_write_height
        database = Path(self.driver.support['indexer_environment']['LAYERX_INDEXER_DB'])
        def finalized():
            connection = sqlite3.connect('file:' + str(database) + '?mode=ro', uri=True, timeout=5)
            try:
                return connection.execute(
                    'SELECT snapshot_id, execution_height, observed_sequence, native_root, state '
                    'FROM ai_snapshot WHERE chain = ? AND program = ? AND projection = 3 '
                    'AND state IS NOT NULL AND observed_sequence >= ? '
                    'ORDER BY observed_sequence DESC LIMIT 1',
                    (bytes.fromhex(self.driver.support['chain']),
                     bytes.fromhex(self.config['program']), required_sequence)).fetchone()
            finally:
                connection.close()
        snapshot_id, height, sequence, root, state = self.driver.processes.wait(finalized)
        commit = self.driver.gateway_rpc('px_getCommit', [str(height)])
        require(commit['height'] == str(height),
                'gateway commit height must match the projected state')
        save(self.driver.directory / ('gateway-commit-' + str(height) + '-' + uuid.uuid4().hex + '.json'), commit)
        snapshot = encoded(self.driver.manifest, 'CommittedState', {
            'commit': commit,
            'state': state.hex(), 'native_root': root.hex(),
            'chain_id': self.runtime.manifest['chain_id'], 'height': height,
            'validators': [{'public_key': bytes(row['pub_key']).hex(),
                            'voting_power': row['voting_power']}
                           for row in self.runtime.manifest['validators']]})
        snapshot.update(snapshot_id=snapshot_id.hex(), observed_sequence=sequence)
        require(4 * (len(state) + 64) <= OWNER_NAMESPACE_BYTES,
                'owner namespace responsibility must cover four times the observed state')
        snapshot.update(namespace_bytes=len(state) + 64,
                        owner_namespace_byte_limit=OWNER_NAMESPACE_BYTES)
        save(self.driver.directory / ('committed-state-' + uuid.uuid4().hex + '.json'), snapshot)
        return snapshot

    def open_room(self):
        self.operation('OpenRoom', {
            **self.config['room'], 'model_digest': self.config['model_digest']},
            self.config['accounts']['host'], self.snapshot()['state'], 'open_room')

    def enrol_peers(self, peers, publish=True):
        manifests = []
        for peer in peers:
            index = self.config['peers'].index(peer)
            fee_account, = self.view(LEDGER, 'accountId', ['bytes32', 'bytes32'],
                                     [bytes.fromhex(peer['peer_key']),
                                      bytes.fromhex(self.config['asset'])], ['bytes32'])
            fee_balance, = self.view(LEDGER, 'balanceOf', ['bytes32', 'bytes32'],
                                     [fee_account, bytes.fromhex(self.config['asset'])], ['uint128'])
            fee_owner, = self.view(LEDGER, 'accountOwner', ['bytes32'],
                                   [fee_account], ['bytes32'])
            save(self.driver.directory / ('peer' + str(index) + '-fee-account.json'),
                 {'account': fee_account.hex(), 'owner': fee_owner.hex(),
                  'balance': str(fee_balance)})
            require(fee_owner.hex() == peer['principal']
                    and fee_balance >= self.config['fee_limit'],
                    'native peer fee account must fund its signed call')
            state = self.snapshot()
            height = int(self.runtime.rpc.request('eth_blockNumber', []), 16)
            worker = encoded(self.driver.manifest, 'DeriveWorker', {
                'market': self.config['market_id'], 'owner': peer['owner'], 'nonce': peer['nonce']})
            peer.update(worker)
            manifest = encoded(self.driver.manifest, 'Manifest', {
                **self.config, **peer, 'market': self.config['market_id'],
                'valid_from': height, 'expiry': height + 256,
                'runtime_digest': self.config['room']['runtime_digest'],
                'endpoint': 'https://localhost:' + str(19000 + index) + '/paxai/v1',
                'accelerator_memory_mib': 0, 'system_memory_mib': 1024,
                'bandwidth_class': 4, 'region_code': 'ZZ',
                'attestation_family': 0, 'runtime_kind': 2})
            peer['metadata_digest'] = manifest['metadata_digest']
            protected_bytes(Path(peer['manifest']), bytes.fromhex(manifest['manifest_v2']))
            self.operation('EnrollWorker', {
                **peer, 'delegate': peer['peer_key'], 'expiry': height + 64},
                self.config['accounts']['host'], state['state'], 'enrol_peers')
            state = self.snapshot()
            self.operation('AcceptEnrollment', {
                **peer, 'state': state['state'],
                'delegate_seed': Path(peer['delegate_seed']).read_bytes().hex()},
                peer, state['state'], 'enrol_peers')
            self.operation('ApproveAdmission', {**peer, 'kind': 'worker'},
                           self.config['accounts']['host'], self.snapshot()['state'], 'enrol_peers')
            self.operation('AdmitWorker', peer, peer, self.snapshot()['state'], 'enrol_peers')
            manifests.append((peer, manifest['manifest_v2']))
        if not publish:
            self.pending_manifests = manifests
            return
        self.publish_manifests(manifests)

    def publish_manifests(self, manifests):
        for peer, manifest in manifests:
            self.operation('PublishMetadata', {'manifest': manifest},
                           peer, self.snapshot()['state'], 'enrol_peers', delegate=True)

    def observe_peers(self):
        for index, peer in enumerate(self.config['peers']):
            observed = self.runtime.rpc.request('px_getInferencePeer', [peer['owner']])
            require(observed['enrolled'] is True and observed['delegate'] == peer['peer_key']
                    and observed['metadata'] == peer['metadata_digest'],
                    'native peer enrolment differs from its signed manifest')
            save(self.driver.directory / ('peer' + str(index) + '-on-chain.json'), observed)

    def enrol_evaluators(self):
        host = self.config['accounts']['host']
        for index in range(3):
            evaluator = self.config['accounts']['evidence' + str(index)]
            terms = {**evaluator, 'kind': 'evaluator', 'nonce': os.urandom(32).hex(),
                     'delegate_seed': Path(evaluator['delegate_seed']).read_bytes().hex()}
            self.operation('ScheduleEvaluator', terms, host, self.snapshot()['state'], 'enrol_peers')
            self.operation('ApproveAdmission', terms, host, self.snapshot()['state'], 'enrol_peers')
            self.operation('AdmitEvaluator', terms, evaluator, self.snapshot()['state'], 'enrol_peers')

    def open_epoch(self):
        self.operation('OPEN_EPOCH', {}, self.config['accounts']['host'],
                       self.snapshot()['state'], 'open_market')

    def advance_epoch(self):
        host = self.config['accounts']['host']
        journey = encoded(self.driver.manifest, 'EpochJourney', {'state': self.snapshot()['state']})
        self.runtime.wait(lambda: int(self.runtime.rpc.request('eth_blockNumber', []), 16)
                          >= journey['settlement_height'], seconds=300)
        self.operation('SEAL_TASK_SET', {}, host, self.snapshot()['state'], 'open_market')
        self.operation('BeginAggregation', {}, host, self.snapshot()['state'], 'open_market')
        for _ in range(32):
            state = self.snapshot()['state']
            progress = encoded(self.driver.manifest, 'AggregationProgress', {'state': state})
            if progress['cursor'] == progress['workers']:
                break
            self.operation('ProcessAggregation', {}, host, state, 'open_market')
        else:
            require(False, 'bootstrap aggregation did not cover its bounded worker roster')
        for _ in range(32):
            state = self.snapshot()['state']
            progress = encoded(self.driver.manifest, 'AggregationProgress', {'state': state})
            if progress['phase'] == 2:
                break
            self.operation('FinalizeAggregation', {}, host, state, 'open_market')
        else:
            require(False, 'bootstrap aggregation did not reach its terminal phase')
        self.runtime.wait(lambda: int(self.runtime.rpc.request('eth_blockNumber', []), 16)
                          >= journey['next_epoch_height'], seconds=300)
        self.open_room()
        self.open_epoch()

    def quote_peers(self):
        for peer in self.config['peers']:
            self.operation('LockBond', {**peer, **self.config['room'],
                                       'amount': self.config['room']['profile']['minimum_bond']},
                           peer, self.snapshot()['state'], 'enrol_peers')
            self.operation('PostBid', {**peer, **self.config['room'],
                                      'capability_digest': peer['metadata_digest'],
                                      'minimum_price_per_thousand_tokens': self.config['price'],
                                      'bond_amount': self.config['room']['profile']['minimum_bond'],
                                      'layer_capacity': 2},
                           peer, self.snapshot()['state'], 'enrol_peers')

    def fund_ask(self):
        account = self.config['accounts']['asker']
        require(int(self.config['price']) % 1000 == 0, 'exact integer per-token ask rate required')
        self.submit('fund-ask-stream', account, PAY,
                    encode_call('streamOpenForAsk',
                                ['bytes32', 'address', 'uint256', 'uint8', 'uint256', 'bytes32'],
                                [bytes.fromhex(self.config['ask_id']),
                                 bytes.fromhex(self.config['accounts']['host']['address'][2:]),
                                 int(self.config['price']) // 1000, 1, 1000000,
                                 bytes.fromhex(self.config['host_meter_key'])]), 'fund_ask_stream')
        stream, = self.view(PAY, 'askStreamOf', ['bytes32'],
                            [bytes.fromhex(self.config['ask_id'])], ['bytes32'])
        require(stream != bytes(32), 'native ask stream must be funded before posting')
        stream_id = int.from_bytes(stream, 'big')
        save(self.driver.directory / 'ask-stream.json', {'ask': self.config['ask_id'], 'stream': stream.hex()})
        require(stream_id < 1 << 128, 'native ask stream identifier exceeds PostAsk u128 interface')
        self.config['stream_id'] = str(stream_id)
        self.operation('PostAsk', {**self.config['room'], 'ask_id': self.config['ask_id'],
                                  'model_digest': self.config['model_digest'], 'stream_id': str(stream_id),
                                  'maximum_price_per_thousand_tokens': self.config['price']},
                       account, self.snapshot()['state'], 'post_ask')

    def open_market(self):
        *_, asset = self.view(PROGRAMS, 'getCurrentFeeSchedule', [], [],
                             ['uint32', *(['uint64'] * 7), 'bytes32'])
        self.config['asset'] = asset.hex()
        host = self.config['accounts']['host']
        parameters = {'program': self.config['program'], 'owner': host['principal'],
                      'asset': asset.hex(), 'refund': host['main_account'],
                      'model_digest': self.config['model_digest'],
                      'commitment_digest': self.config['commitment_digest'],
                      'epoch_budget_cap': '1000000', 'minimum_epoch_funding': '1'}
        market = encoded(self.driver.manifest, 'CreateMarket', parameters)
        self.config.update(market_id=market['market'], rewards=market['rewards'])
        registration = encode_call('registerAccount', ['bytes32', 'uint8', 'bytes32', 'bytes'],
                                   [bytes.fromhex(self.config['program']), 1, asset, b'paxai/rewards/v1'])
        self.submit('register-rewards', host, PROGRAMS, registration, 'open_market')
        self.operation('CreateMarket', parameters, host, phase='open_market')

    def fund_market(self):
        host = self.config['accounts']['host']
        self.operation('FUND', {'amount': '1000000', 'refund': host['main_account']},
                       host, self.snapshot()['state'], 'open_market')


class InferenceRoomDriver:
    def __init__(self, directory, config, build_directory, manifest):
        self.directory = Path(directory)
        self.config = config
        self.build_directory = build_directory
        self.manifest = manifest
        self.runtime = RuntimeFixture.for_inference_room(
            self.directory / 'runtime', manifest['source_revision'],
            manifest['paxd'], config['accounts'])
        self.runtime.config["ledger_encoder"] = manifest["support"]["chain-evidence"]
        self.runtime.config["inference_program"] = config["program"]
        self.runtime.config["inference_reader"] = config["accounts"]["host"]["address"]
        self.processes = envelope.ParticipantProcesses(self.directory)
        self.results = []

    def native_source(self):
        """Read the product source process, retaining its first refused response."""
        url = self.support['source_url']
        parsed = urlsplit(url)
        path = '/v1/ai/programs/' + self.config['program'] + '/committed-read?after=0'
        connection = http.client.HTTPConnection(parsed.hostname, parsed.port, timeout=10)
        try:
            connection.request('GET', path)
            response = connection.getresponse()
            body = response.read(8_388_609)
            record = {'endpoint': url + path, 'status': response.status,
                      'body': body.decode('utf-8', errors='replace')}
        finally:
            connection.close()
        save(self.directory / ('native-ai-source-' + uuid.uuid4().hex + '.json'), record)
        require(len(body) <= 8_388_608, 'bounded native AI committed read')
        require(response.status == 200, 'AI source refused committed read: '
                + str(response.status) + ' ' + record['body'][:4096]
                + '; services/platform/hosted/indexer/src/ai_source.rs')
        document = json.loads(body)
        require(document['program'] == list(bytes.fromhex(self.config['program']))
                and document['chain'] == list(bytes.fromhex(self.support['chain'])),
                'source returned another program or chain')
        return document

    def configure_support(self):
        ports = reserve_ports(5)
        source, indexer, redis, gateway, consensus_tls = [sock.getsockname()[1] for sock in ports]
        self.support_ports = dict(zip(('source', 'indexer', 'redis', 'gateway', 'consensus-tls'), ports))
        ca = CaMaterial(self.directory)
        self.ca = ca
        certificate, private_key = ca.issue('inference-room-gateway', [], False, ['localhost'])
        cert_der, key_der = self.directory / 'server.der', self.directory / 'server-key.der'
        commands = [
            ['openssl', 'x509', '-in', str(certificate), '-outform', 'DER', '-out', str(cert_der)],
            ['openssl', 'pkcs8', '-topk8', '-nocrypt', '-in', str(private_key),
             '-outform', 'DER', '-out', str(key_der)],
        ]
        with (self.directory / 'tls.log').open('xb') as log:
            for command in commands:
                result = subprocess.run(command, cwd=ROOT, stdout=log, stderr=log, timeout=30)
                require(result.returncode == 0, 'private TLS generation failed; inspect tls.log')
        chain = hashlib.sha256(b'PAXAI/chain-domain/v1' + (125).to_bytes(8, 'big')).hexdigest()
        validators = self.runtime.manifest['validators']
        consensus = self.runtime.config['consensus_rpc_url']
        source_url = 'http://127.0.0.1:' + str(source)
        database = str(self.directory / 'indexer.sqlite')
        ai_config = self.directory / 'indexer-ai.json'
        save(ai_config, {'chain': list(bytes.fromhex(chain)),
                         'program': list(bytes.fromhex(self.config['program'])),
                         'consensus_chain_id': self.runtime.manifest['chain_id'],
                         'validators': validators, 'source': source_url})
        authority = self.directory / 'state-validators.json'
        save(authority, {'chain_id': self.runtime.manifest['chain_id'],
                         'validators': [{'public_key': bytes(v['pub_key']).hex(),
                                         'voting_power': v['voting_power']} for v in validators]})
        cursor = self.directory / 'cursor.key'
        protected_bytes(cursor, os.urandom(32).hex().encode())
        username, password = 'gateway', os.urandom(32).hex()
        for name, value in (('redis-username', username), ('redis-password', password)):
            protected_bytes(self.directory / name, value.encode())
        redis_directory = self.directory / 'redis'
        redis_directory.mkdir(mode=0o700)
        redis_config = self.directory / 'redis.conf'
        protected_bytes(redis_config, ('bind 127.0.0.1\nport 0\ntls-port ' + str(redis)
                        + '\ntls-cert-file ' + str(certificate) + '\ntls-key-file ' + str(private_key)
                        + '\ntls-ca-cert-file ' + str(ca.ca_pem) + '\ntls-auth-clients no'
                        + '\nuser default off\nuser gateway on >' + password + ' ~* &* +@all'
                        + '\nappendonly yes\nappendfsync always\nsave ""\ndir '
                        + str(redis_directory) + '\nprotected-mode yes\n').encode())
        source_env = {'LAYERX_AI_SOURCE_LISTEN': '127.0.0.1:' + str(source),
                      'LAYERX_AI_SOURCE_NODE_URL': consensus, 'LAYERX_AI_SOURCE_CHAIN': chain,
                      'LAYERX_AI_SOURCE_PROGRAM': self.config['program']}
        indexer_env = {'LAYERX_INDEXER_LISTEN': '127.0.0.1:' + str(indexer),
                       'LAYERX_INDEXER_DB': database, 'LAYERX_INDEXER_AI_CONFIG': str(ai_config),
                       'LAYERX_INDEXER_EVM_URL': self.runtime.rpc_url,
                       'LAYERX_INDEXER_EVM_CHAIN_ID': '125',
                       'LAYERX_INDEXER_TLS_CERT_DER': str(cert_der),
                       'LAYERX_INDEXER_TLS_KEY_DER': str(key_der)}
        gateway_env = {'LAYERX_GATEWAY_LISTEN': '127.0.0.1:' + str(gateway),
                       'LAYERX_GATEWAY_LISTENER': 'tls',
                       'LAYERX_GATEWAY_TLS_CERT_DER': str(cert_der),
                       'LAYERX_GATEWAY_TLS_KEY_DER': str(key_der),
                       'LAYERX_GATEWAY_OUTBOUND_CA_DER': str(ca.ca_der),
                       'LAYERX_GATEWAY_NETWORK_ID': self.runtime.manifest['chain_id'],
                       'LAYERX_GATEWAY_LXP_WIRE_VERSION': '3',
                       'LAYERX_GATEWAY_PROTOCOL_NETWORK_ID': '125',
                       'LAYERX_GATEWAY_REDIS_URL': 'rediss://localhost:' + str(redis),
                       'LAYERX_GATEWAY_REDIS_USERNAME_FILE': str(self.directory / 'redis-username'),
                       'LAYERX_GATEWAY_REDIS_PASSWORD_FILE': str(self.directory / 'redis-password'),
                       'LAYERX_GATEWAY_AI_PROJECTION_DB': database,
                       'LAYERX_GATEWAY_AI_CURSOR_KEY_FILE': str(cursor),
                       'LAYERX_GATEWAY_AI_CURSOR_GENERATION': '1',
                       'LAYERX_GATEWAY_STATE_VALIDATORS_FILE': str(authority),
                       'LAYERX_GATEWAY_CONSENSUS_RPC_URL': 'https://localhost:' + str(consensus_tls)}
        self.support = {'chain': chain, 'source_url': source_url,
                        'indexer_url': 'https://localhost:' + str(indexer),
                        'gateway_url': 'https://localhost:' + str(gateway),
                        'source_environment': source_env, 'indexer_environment': indexer_env,
                        'gateway_environment': gateway_env, 'redis_config': str(redis_config),
                        'redis_username': username, 'redis_password': password,
                        'consensus_tls': {'port': consensus_tls, 'upstream': consensus,
                                          'certificate': str(certificate), 'key': str(private_key)}}
        save(self.directory / 'support-configuration.json', self.support)
        self.tls_context = ssl.create_default_context(cafile=str(ca.ca_pem))

    def support_http(self, url, authorization=None):
        parsed = urlsplit(url)
        connection = http.client.HTTPSConnection(parsed.hostname, parsed.port, timeout=5,
                                                  context=self.tls_context)
        try:
            headers = {} if authorization is None else {'Authorization': authorization}
            connection.request('GET', parsed.path or '/', headers=headers)
            response = connection.getresponse()
            body = response.read(1_048_577)
            require(len(body) <= 1_048_576, 'bounded private support response')
            return response.status, json.loads(body)
        finally:
            connection.close()

    def gateway_rpc(self, method, params):
        parsed = urlsplit(self.support['gateway_url'])
        connection = http.client.HTTPSConnection(parsed.hostname, parsed.port, timeout=10,
                                                  context=self.tls_context)
        request = {'jsonrpc': '2.0', 'id': 1, 'method': method, 'params': params}
        try:
            connection.request('POST', '/', json.dumps(request).encode(),
                               {'Content-Type': 'application/json',
                                'Authorization': self.gateway_credential})
            response = connection.getresponse()
            raw = response.read(1_048_577)
            require(response.status == 200 and len(raw) <= 1_048_576
                    and response.getheader('Content-Type', '').split(';')[0] == 'application/json',
                    'bounded gateway RPC response required')
            document = json.loads(raw)
            require(document.get('jsonrpc') == '2.0' and document.get('id') == 1,
                    'gateway RPC envelope required')
            require('error' not in document, 'gateway RPC refusal: ' + str(document.get('error')))
            require(isinstance(document.get('result'), dict), 'gateway RPC result required')
            return document['result']
        finally:
            connection.close()

    def materialize(self):
        reference = load(self.build_directory / 'model/reference.json', 'scripted reference')
        ports = reserve_ports(5)
        signal_port, completion_port, router_port, model_port, program_port = [sock.getsockname()[1] for sock in ports]
        self.ports = ports
        values = {'run': str(self.directory), 'build': str(self.build_directory), 'rpc_url': self.runtime.rpc_url,
                  'signal_address': '127.0.0.1:' + str(signal_port),
                  'completion_address': '127.0.0.1:' + str(completion_port),
                  'completion_socket': str(self.directory / 'completion.sock'),
                  'completion_receipts': str(self.directory / 'completion-receipts.jsonl'),
                  'journal': str(self.directory / 'journal'),
                  'room_profile': str(self.directory / 'room-profile.json'),
                  'model_digest': reference['model_digest']}
        self.values = values
        (self.directory / 'journal').mkdir(mode=0o700)
        host = self.config['accounts']['host']
        authority_endpoint, authority_token = self.human_authority()
        files = self.directory / 'agent-files'
        files.mkdir(mode=0o700)
        snapshot = GeneratedRoomWrites(self).snapshot()
        protected_bytes(files / 'trust-root.bin', bytes.fromhex(snapshot['trust_root']))
        protected_bytes(files / 'policy.txt', b'version=inference-room-v1\nsteps=1\nrule=deny-general-writes,deny\n')
        native_policy = {'schema': 'layerx.native-program-policy.v1', 'version': 'inference-room-v1',
                         'network_id': 125, 'step_limit': 1,
                         'rules': [{'id': 'deny-general-program-writes', 'effect': 'Deny',
                                    'activity': {'module': 9, 'ordinal': 3}, 'scope': {'kind': 'Operation'},
                                    'required_approval': False, 'purpose_commitment': None}]}
        save(files / 'native-policy.json', {'version': 'layerx.native-program-policy-source.v1',
                                           'tenant': 'inference-room', 'policy': json.dumps(native_policy)})
        protected_bytes(files / 'session-operator.key', os.urandom(32))
        for name in ('session-keys', 'clock'):
            (files / name).mkdir(mode=0o700)
        price = int(self.runtime.rpc.request('eth_gasPrice', []), 16)
        terms = {**self.custody.signers['host'], 'journal': values['journal'], 'actor_did': host['did'],
                 'layer_count': 4, 'layer_memory_mib': 1, 'validity_blocks': 256,
                 'fee_limit': self.config['execution_fee_limit'], 'gas_limit': 30000000,
                 'max_fee_per_gas': price * 2, 'max_priority_fee_per_gas': 0, 'resources': RESOURCES,
                 'laps': {'batch_size': 1, 'host_meter_seed_source': str(self.directory / 'keys/host-meter.ed25519'),
                          'evidence_signers': [self.custody.signers['evidence' + str(index)] for index in range(3)]}}
        save(files / 'deals.json', terms)
        agent = {'environment': {
            'LAYERX_AGENT_NODE_ENDPOINT': self.runtime.rpc_url,
            'LAYERX_AGENT_CONSENSUS_ENDPOINT': self.runtime.config['consensus_rpc_url'],
            'LAYERX_EXPECTED_CHAIN_ID': '125',
            'LAYERX_AGENT_PROGRAM_LISTEN': '127.0.0.1:' + str(program_port),
            'LAYERX_AGENT_PROGRAM_BEARER_TOKEN': os.urandom(32).hex(),
            'LAYERX_AGENT_NODE_BEARER_TOKEN': os.urandom(32).hex(),
            'LAYERX_AGENT_AUTHORITY_BEARER_TOKEN': self.gateway_credential,
            'LAYERX_AGENT_AUTHORITY_ENDPOINT': self.support['gateway_url'],
            'LAYERX_AGENT_AUTHORITY_CA_DER': str(self.ca.ca_der),
            'LAYERX_AGENT_AUTHORITY_REPLICA_ID': self.config['program'],
            'LAYERX_AGENT_CHAIN_TRUST_ROOT': str(files / 'trust-root.bin'),
            'LAYERX_AGENT_DEPLOYMENT_JOURNAL': str(self.directory / 'deployment-journal'),
            'LAYERX_AGENT_PROGRAM_PROBE_ID': self.config['program'],
            'LAYERX_AGENT_PROGRAM_MAX_STALENESS_MS': '120000',
            'LAYERX_AGENT_INFERENCE_DEALS': str(files / 'deals.json'),
            'LAYERX_AGENT_HUMAN_PEERS': 'uid=' + str(os.geteuid()) + ';tenant=inference-room;principal=' + host['did'],
            'LAYERX_POLICY_SOURCES': 'inference-room:' + str(files / 'policy.txt'),
            'LAYERX_NATIVE_POLICY_SOURCES': 'inference-room:' + str(files / 'native-policy.json'),
            'LAYERX_AGENT_NODE_RPC_URL': self.runtime.rpc_url,
            'LAYERX_AGENT_NODE_CHAIN_ID': '125',
            'LAYERX_AGENT_HUMAN_PROTOCOL_VERSION': '3', 'LAYERX_AGENT_HUMAN_NETWORK_ID': '125',
            'LAYERX_AGENT_HUMAN_AUTHORITY_ENDPOINT': authority_endpoint,
            'LAYERX_AGENT_HUMAN_AUTHORITY_BEARER': authority_token,
            'LAYERX_AGENT_HUMAN_AUTHORITY_CA_DER': str(self.ca.ca_der),
            'LAYERX_AGENT_HUMAN_AUTHORITY_MAX_BYTES': '1048576',
            'LAYERX_AGENT_HUMAN_DEADLINE_MS': '10000',
            'LAYERX_AGENT_HUMAN_STORE': str(files / 'human.sqlite'),
            'LAYERX_AGENT_HUMAN_SOCKET': str(files / 'human.sock'),
            'LAYERX_AGENT_HUMAN_SOCKET_UID': str(os.geteuid()),
            'LAYERX_AGENT_HUMAN_SOCKET_GID': str(os.getegid()),
            'LAYERX_AGENT_HUMAN_SOCKET_MODE': '0600',
            'LAYERX_AGENT_HUMAN_SESSION_KEY_ROOT': str(files / 'session-keys'),
            'LAYERX_AGENT_HUMAN_SESSION_OPERATOR_SECRET_FILE': str(files / 'session-operator.key'),
            'LAYERX_AGENT_HUMAN_MAX_PAYLOAD_BYTES': '1048576',
            'LAYERX_AGENT_HUMAN_TIMESTAMP_SPAN': '120000',
            'LAYERX_AGENT_HUMAN_MAX_FRAME_BYTES': '1048576',
            'LAYERX_AGENT_HUMAN_MAX_CONNECTIONS': '16', 'LAYERX_AGENT_HUMAN_MAX_STREAMS': '16',
            'LAYERX_AGENT_HUMAN_MAX_QUEUED_BYTES': '4194304',
            'LAYERX_AGENT_HUMAN_LIMIT_SCOPE': 'tenant',
            'LAYERX_AGENT_HUMAN_LIMIT_SCOPE_ID': hashlib.sha256(b'inference-room').hexdigest(),
            'LAYERX_AGENT_HUMAN_LIMIT_ID': os.urandom(16).hex(),
            'LAYERX_AGENT_HUMAN_LIMIT_NAME': 'inference-room',
            'LAYERX_AGENT_HUMAN_LIMIT_CEILING': '1000000', 'LAYERX_AGENT_HUMAN_LIMIT_CONSUMED': '0'}}
        require(agent['environment']['LAYERX_AGENT_NODE_ENDPOINT'].rstrip('/') == self.runtime.rpc_url.rstrip('/'),
                'agent must use the owned chain endpoint')
        require(agent['environment']['LAYERX_EXPECTED_CHAIN_ID'] == '125', 'agent must bind chain 125')
        save(files / 'environment.json', agent['environment'])
        profile = self.config['room_offer']
        require(profile['room_id'] == list(bytes.fromhex(self.config['room']['room_id']))
                and profile['model_digest'] == list(bytes.fromhex(reference['model_digest']))
                and profile['runtime_digest'] == list(bytes.fromhex(self.config['room']['runtime_digest'])),
                'room offer must bind the built scripted model and native room')
        save(Path(values['room_profile']), profile)
        self.agent_env = dict(self.runtime.env)
        for name, value in agent['environment'].items():
            require(name.startswith('LAYERX_') and isinstance(value, str), 'explicit agent setting required')
            self.agent_env[name] = value
        self.agent_env.update(LAYERX_AGENT_MODE='inference-market',
                              LAYERX_AGENT_INFERENCE_GATEWAY_ENDPOINT=self.support['gateway_url'],
                              LAYERX_AGENT_INFERENCE_GATEWAY_API_KEY=self.gateway_credential,
                              LAYERX_AGENT_INFERENCE_GATEWAY_CA_FILE=str(self.ca.ca_pem),
                              LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN=values['signal_address'],
                              LAYERX_AGENT_INFERENCE_SERVE_LISTEN=values['completion_address'],
                              LAYERX_AGENT_INFERENCE_ROOM_PROFILE=values['room_profile'],
                              LAYERX_AGENT_INFERENCE_MODEL_DIGEST=reference['model_digest'],
                              LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST=self.config['room']['runtime_digest'],
                              LAYERX_AGENT_INFERENCE_COMPLETION_SOCKET=values['completion_socket'],
                              LAYERX_AGENT_INFERENCE_COMPLETION_RECEIPTS=values['completion_receipts'])
        terms = load(self.agent_env['LAYERX_AGENT_INFERENCE_DEALS'], 'materialized inference deal terms')
        require(terms['journal'] == values['journal'], 'daemon negotiation evidence must remain in the run')
        seed = Path(terms['laps']['host_meter_seed_source']).read_bytes()
        require(len(seed) == 32, 'native host meter seed')
        key = envelope.Ed25519PrivateKey.from_private_bytes(seed)
        host_meter_key = list(key.public_key().public_bytes(envelope.Encoding.Raw, envelope.PublicFormat.Raw))
        peers = [{**peer, 'transport_token': os.urandom(32).hex()} for peer in self.config['peers']]
        native = {**values, 'room': self.config['room'], 'peers': peers, 'ask_id': self.config['ask_id'],
                  'price': self.config['price'], 'model_directory': str(self.build_directory / 'model'),
                  'model_port': model_port, 'router_port': router_port,
                  'model_url': 'http://127.0.0.1:' + str(model_port),
                  'completion_url': 'http://' + values['completion_address'],
                  'signal_url': 'ws://' + values['signal_address'] + '/',
                  'agent_bearer': self.agent_env['LAYERX_AGENT_PROGRAM_BEARER_TOKEN'], 'host_meter_key': host_meter_key}
        self.native = native
        self.native_config = self.directory / 'native.json'
        save(self.native_config, native)

    def human_authority(self):
        directory = self.directory / 'human-authority'
        directory.mkdir(mode=0o700)
        state = directory / 'state'
        state.mkdir(mode=0o700)
        tls = der_pair(self.ca, directory, 'human-authority')
        human_token, general_token, replica_token = [os.urandom(32).hex() for _ in range(3)]
        for name, token in (('human', human_token), ('general', general_token), ('replica', replica_token)):
            protected_bytes(directory / (name + '.token'), token.encode())
        host = self.config['accounts']['host']
        save(directory / 'policy.json', {'principals': [{
            'tenant': 'inference-room', 'principal': host['did'],
            'account_id': host['main_account'], 'asset_id': self.config['asset'],
            'activities': [], 'budgets': [], 'maximum_age_seconds': 120,
            'maximum_age_sequences': 256, 'identities': []}]})
        save(directory / 'registry.json', {'schema_version': 2,
            'assets': [{'asset': self.config['asset'], 'currency': 'T', 'symbol': 'T', 'decimals': 6}],
            'modules': [{'module': 9, 'ordinals': [1, 2, 3]}]})
        genesis = json.loads((self.runtime.home / 'config/genesis.json').read_text())
        pins = genesis['app_state']['sequencer']['receipt_keys']
        require(len(pins) == 1, 'isolated authority requires its pinned native receipt key')
        save(directory / 'receipt-keys.json', pins)
        key = pins[0]
        ports = reserve_ports(2)
        port, replica_port = [sock.getsockname()[1] for sock in ports]
        self.ports.extend(ports[1:])
        endpoint = 'https://localhost:' + str(port)
        environment = {
            'LAYERX_AUTHORITY_LISTEN': '127.0.0.1:' + str(port),
            'LAYERX_AUTHORITY_TLS_CERT_DER': str(tls['cert_der']),
            'LAYERX_AUTHORITY_TLS_KEY_DER': str(tls['key_der']),
            'LAYERX_AUTHORITY_TOKEN_FILES': str(directory / 'general.token'),
            'LAYERX_AUTHORITY_REPLICA_URL': 'http://127.0.0.1:' + str(replica_port),
            'LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE': str(directory / 'replica.token'),
            'LAYERX_AUTHORITY_REPLICA_ID': self.config['program'],
            'LAYERX_AUTHORITY_CHAIN_RPC_URL': self.runtime.rpc_url,
            'LAYERX_AUTHORITY_RECEIPT_KEYS_FILE': str(directory / 'receipt-keys.json'),
            'LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID': '125',
            'LAYERX_AUTHORITY_NETWORK_ID': self.runtime.manifest['chain_id'],
            'LAYERX_AUTHORITY_WIRE_VERSION': '3',
            'LAYERX_AUTHORITY_SEQUENCER_ID': base64.b64decode(key['key_id']).hex(),
            'LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY': base64.b64decode(key['public_key']).hex(),
            'LAYERX_AUTHORITY_FIRST_BATCH': str(key['first_height']),
            'LAYERX_AUTHORITY_LAST_BATCH': str(key['last_height']),
            'LAYERX_AUTHORITY_HUMAN_AGENT_TOKEN_FILE': str(directory / 'human.token'),
            'LAYERX_AUTHORITY_PRINCIPAL_POLICY_FILE': str(directory / 'policy.json'),
            'LAYERX_AUTHORITY_CORE_CLOCK_HORIZON': '256',
            'LAYERX_AUTHORITY_STATE_ROOT': str(state),
            'LAYERX_AUTHORITY_HUMAN_AGENT_TENANT': 'inference-room',
            'LAYERX_AUTHORITY_HUMAN_AGENT_PRINCIPAL': host['did'],
            'LAYERX_AUTHORITY_MODULE_REGISTRY_FILE': str(directory / 'registry.json')}
        ports[0].close()
        self.processes.start('human-authority', [self.manifest['support']['layerx-receipt-authority']],
                             dict(self.runtime.env, **environment))
        def registry_ready():
            from urllib.parse import urlencode
            query = urlencode({'tenant': 'inference-room', 'principal': host['did']})
            status, _, body = https('GET', endpoint + '/v1/agent/registry?' + query, self.ca,
                                   headers={'Authorization': 'Bearer ' + human_token})
            return status == 200 and json.loads(body).get('modules') == [
                {'module_id': 9, 'activity_types': [(9 << 16) | ordinal for ordinal in (1, 2, 3)]}]
        self.processes.wait(registry_ready)
        return endpoint, human_token

    def support_services(self):
        self.configure_support()
        GeneratedRoomWrites(self).next_committed_height()
        for name, key in (('layerx-ai-source', 'source'), ('layerx-indexer', 'indexer')):
            self.support_ports[key].close()
            self.processes.start(key, [self.manifest['support'][name]],
                                 dict(self.runtime.env, **self.support[key + '_environment']))
            if key == 'source':
                parsed = urlsplit(self.support['source_url'])
                def listening():
                    with socket.create_connection((parsed.hostname, parsed.port), timeout=.5):
                        return True
                self.processes.wait(listening)
                self.native_source()
            else:
                self.processes.wait(lambda: self.support_http(self.support['indexer_url'] + '/readyz')[0] == 200)
        self.support_ports['redis'].close()
        redis = shutil.which('redis-server')
        require(redis is not None, 'redis-server prerequisite missing')
        self.processes.start('redis', [redis, self.support['redis_config']], self.runtime.env)
        redis_client = shutil.which('redis-cli')
        require(redis_client is not None, 'redis-cli prerequisite missing')
        port = urlsplit(self.support['gateway_environment']['LAYERX_GATEWAY_REDIS_URL']).port
        redis_env = dict(self.runtime.env, REDISCLI_AUTH=self.support['redis_password'])
        def redis_ready():
            result = subprocess.run([redis_client, '--tls', '--cacert', str(self.ca.ca_pem),
                                     '--user', self.support['redis_username'], '-h', 'localhost',
                                     '-p', str(port), 'PING'], env=redis_env, cwd=ROOT,
                                    capture_output=True, text=True, timeout=5)
            return result.returncode == 0 and result.stdout.strip() == 'PONG'
        self.processes.wait(redis_ready)
        host = self.config['accounts']['host']
        key = encoded(self.manifest, 'GatewayKey', {
            'redis_url': self.support['gateway_environment']['LAYERX_GATEWAY_REDIS_URL'],
            'ca_der': str(self.ca.ca_der), 'username': self.support['redis_username'],
            'password': self.support['redis_password'], 'principal': host['principal'],
            'public_key': host['peer_key']})
        save(self.directory / 'gateway-key.json', key)
        self.gateway_credential = 'LayerX-Key ' + key['credential']
        self.support_ports['consensus-tls'].close()
        self.processes.start('consensus-tls', [sys.executable, str(Path(__file__).resolve()),
                                              '--tls-tunnel', self.directory / 'support-configuration.json'],
                             self.runtime.env)
        self.processes.wait(lambda: self.support_http(
            self.support['gateway_environment']['LAYERX_GATEWAY_CONSENSUS_RPC_URL'] + '/status')[0] == 200)
        self.support_ports['gateway'].close()
        self.processes.start('gateway', [self.manifest['support']['layerx-gateway']],
                             dict(self.runtime.env, **self.support['gateway_environment']))
        self.processes.wait(lambda: self.support_http(self.support['gateway_url'] + '/livez')[0] == 200)
        def published():
            status, body = self.support_http(self.support['gateway_url'] + '/v1/ai/markets/'
                                             + self.config['market_id'] + '/snapshot', self.gateway_credential)
            save(self.directory / ('gateway-observation-' + uuid.uuid4().hex + '.json'),
                 {'status': status, 'body': body})
            return status == 200 and body.get('result', {}).get('committed') is True
        self.processes.wait(published)

    def run(self):
        self.runtime.generate()
        self.runtime.start()
        self.runtime.readiness()
        require(self.runtime.rpc.request('eth_chainId', []) == '0x7d', 'room requires chain 125')
        generated = GeneratedRoomWrites(self)
        generated.bind_accounts()
        version, ceiling, _ = generated.view(PROGRAMS, 'quoteCallFee', ['uint64[7]'],
                                             [RESOURCES], ['uint32', 'uint128', 'bytes32'])
        require(version > 0, 'governed room execution fee quote required')
        self.config['execution_fee_limit'] = ceiling + 1_000_000
        self.custody = RoomCustody(self)
        generated.deploy()
        generated.open_market()
        self.support_services()
        generated.enrol_evaluators()
        generated.enrol_peers(self.config['peers'][:1], publish=False)
        generated.fund_market()
        generated.open_epoch()
        generated.publish_manifests(generated.pending_manifests)
        generated.enrol_peers(self.config['peers'][1:])
        generated.advance_epoch()
        generated.quote_peers()
        generated.observe_peers()
        self.materialize()
        room = self.runtime.rpc.request('px_getInferenceRoom', [self.config['room']['room_id']])
        profile = {key: self.config['room']['profile'][key] for key in (
            'minimum_bond', 'canary_ratio_bps', 'lap_size_tokens', 'challenge_window_blocks',
            'slash_fraction_bps', 'host_share_bps', 'protocol_share_bps', 'settlement_account')}
        for key in ('lap_size_tokens', 'challenge_window_blocks'):
            profile[key] = str(profile[key])
        require(room['state'] == 0 and room['model_digest'] == self.native['model_digest']
                and room['runtime_digest'] == self.config['room']['runtime_digest']
                and room['host_key'] == self.config['room']['host_peer_key']
                and room['profile'] == profile, 'opened native room differs from profile')
        save(self.directory / 'room.json', room)
        for sock in self.ports:
            sock.close()
        self.processes.start('host', ['node', self.manifest['native'], 'host', self.native_config], self.runtime.env)
        self.processes.wait(lambda: (self.directory / 'host-ready.json').exists())
        self.processes.start('agentd', [self.manifest['support']['layerx-runtime-clock'],
                                      '--', self.manifest['agent']], self.agent_env)
        self.processes.wait(lambda: http_json(self.native['completion_url'] + '/healthz')[0] == 200)
        for index in range(2):
            self.processes.start('peer' + str(index), ['node', self.manifest['native'], 'peer', self.native_config, str(index)], self.runtime.env)
        self.processes.wait(lambda: all((self.directory / ('peer' + str(i) + '-offered.json')).exists() for i in range(2)))
        ask = {**self.config, 'rpc_url': self.runtime.rpc_url,
               'run': str(self.directory), 'completion_url': self.native['completion_url'],
               'runtime_manifest': self.runtime.manifest, 'artifacts': self.manifest,
               'support': self.support, 'gateway_ca': str(self.ca.ca_pem),
               'gateway_credential': self.gateway_credential}
        ask_path = self.directory / 'asker.json'
        save(ask_path, ask)
        descriptor = os.open(self.directory / 'asker.log', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(descriptor, 'wb') as log:
            result = subprocess.run([sys.executable, str(Path(__file__).resolve()), '--asker', str(ask_path)],
                                    cwd=ROOT, env=self.runtime.env, stdout=log, stderr=log, timeout=180)
        require(result.returncode == 0, 'asker failed; inspect ' + str(self.directory / 'asker.log'))
        self.processes.check()
        expected = load(self.build_directory / 'model/reference.json', 'serial scripted model output')
        actual = load(self.directory / 'model-output.json', 'distributed scripted model output')
        answer = load(self.directory / 'answer.json', 'actual completion response')
        require(actual['tokens'] == expected['tokens'], 'distributed model output differs token for token')
        content = answer['body']['choices'][0]['message']['content']
        require(answer['status'] == 200 and content == ''.join(piece['text'] for piece in expected['tokens'])
                and [int(item) for item in re.findall(r'\[([0-9]+)\]', content)]
                == [piece['token_id'] for piece in expected['tokens']]
                and answer['body']['usage'] == {'prompt_tokens': 1, 'completion_tokens': 4, 'total_tokens': 5},
                'completion answer differs from scripted tokens or exact usage')
        self.results.append({'case': 'scripted_completion_token_equality', 'passed': True})
        deal = load(self.directory / 'peer0-deal.json', 'first native peer deal')
        require(deal == load(self.directory / 'peer1-deal.json', 'second native peer deal'), 'peers accepted different deals')
        identifier = bytes(deal['deal_id']).hex()
        recorded = self.runtime.rpc.request('px_getInferenceDeal', [identifier])
        layers = [{'bid_id': bytes(split['bid_id']).hex(), 'first_layer': split['first_layer'],
                   'end_layer': split['end_layer']} for split in deal['plan']['layers']]
        require(recorded['deal_id'] == identifier and recorded['room_id'] == self.config['room']['room_id']
                and recorded['ask_id'] == self.config['ask_id'] and recorded['layers'] == layers
                and len(layers) == 2 and recorded['roster_digest'] == bytes(deal['roster_digest']).hex()
                and recorded['agreed_price_per_thousand_tokens'] == str(deal['price']), 'two-peer deal differs from native chain record')
        require({bytes(split['peer_key']).hex() for split in deal['plan']['layers']}
                == {peer['peer_key'] for peer in self.config['peers']}, 'chain deal does not bind both actual native peers')
        for index, peer in enumerate(self.config['peers']):
            ack = load(self.directory / ('peer' + str(index) + '-countersign.json'), 'native peer countersignature')
            require(ack['deal_id'] == deal['deal_id'] and ack['lap_index'] == 0, 'native peer did not countersign this deal lap')
        save(self.directory / 'deal-on-chain.json', recorded)
        generated.snapshot()
        self.results.append({'case': 'two_native_peers_recorded_on_chain', 'passed': True})
        save(self.directory / 'case-results.json', self.results)

    def stop(self):
        active_error = sys.exception()
        self.processes.stop()
        for sock in [*getattr(self, 'ports', []), *getattr(self, 'support_ports', {}).values(),
                     *getattr(self, 'custody_ports', [])]:
            sock.close()
        try:
            if self.runtime.process is not None and self.runtime.process.poll() is None:
                try:
                    self.runtime.collect_chain_writes()
                except Exception as error:
                    save(self.directory / 'chain-write-collection-failure.json', {'error': str(error)})
                    if active_error is None:
                        raise
                    print('Chain write collection also failed: ' + str(error), file=sys.stderr, flush=True)
        finally:
            self.runtime.cleanup()


def asker(path):
    config = load(path, 'native asker configuration')
    require(config['schema'] == SCHEMA, 'native asker schema')
    rpc = RootRpc(config['rpc_url'])
    directory = Path(config['run'])
    # Use the same receipt/evidence implementation without creating a second chain.
    deadline = time.monotonic() + 120
    def wait(predicate, seconds=120):
        until = min(deadline, time.monotonic() + seconds)
        while time.monotonic() < until:
            if predicate():
                return
            time.sleep(.1)
        raise RuntimeError('asker chain/deal deadline')
    runtime = ChainWriter(directory / 'runtime', rpc, wait)
    runtime.config = {'consensus_rpc_url': config['runtime_manifest']['consensus_rpc_url']}
    runtime.rpc_url = config['rpc_url']
    runtime.manifest = config['runtime_manifest']
    driver = SimpleNamespace(directory=directory, config=config, runtime=runtime,
                             manifest=config['artifacts'],
                             support=config['support'],
                             processes=envelope.ParticipantProcesses(directory),
                             tls_context=ssl.create_default_context(cafile=config['gateway_ca']),
                             gateway_credential=config['gateway_credential'],
                             last_write_height=int(rpc.request('eth_blockNumber', []), 16) - 1)
    driver.gateway_rpc = lambda method, params: InferenceRoomDriver.gateway_rpc(driver, method, params)
    GeneratedRoomWrites(driver).fund_ask()
    wait(lambda: all((directory / ('peer' + str(index) + '-live.json')).exists() for index in range(2)))
    request = {'model': config['model_digest'], 'messages': [{'role': 'user', 'content': '5'}],
               'max_tokens': 4, 'stream': False}
    save(directory / 'completion-request.json', {'ask_id': config['ask_id'], 'body': request})
    status, body = http_json(config['completion_url'] + '/v1/chat/completions', 'POST', request, config['ask_id'])
    save(directory / 'answer.json', {'status': status, 'body': body})
    require(status == 200, 'funded native ask completion refused')


def identity_issuer(path):
    config = load(path, 'private identity issuer')
    document = Path(config['jwks']).read_bytes()

    class Issuer(BaseHTTPRequestHandler):
        def do_GET(self):
            require(self.path == '/', 'identity issuer key route')
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(document)))
            self.end_headers()
            self.wfile.write(document)

        def log_message(self, *arguments):
            pass

    with ThreadingHTTPServer(('127.0.0.1', config['port']), Issuer) as server:
        server.serve_forever()


def tls_tunnel(path):
    """Terminate private TLS and forward HTTP bodies to the owned consensus RPC."""
    config = load(path, 'private support configuration')['consensus_tls']
    upstream = RootRpc(config['upstream']).url
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.minimum_version = ssl.TLSVersion.TLSv1_2
    context.load_cert_chain(config['certificate'], config['key'])

    class Tunnel(BaseHTTPRequestHandler):
        protocol_version = 'HTTP/1.1'

        def setup(self):
            self.request.settimeout(10)
            self.request = context.wrap_socket(self.request, server_side=True)
            super().setup()

        def forward(self):
            require(self.headers.get('Transfer-Encoding') is None,
                    'consensus TLS request must have a bounded content length')
            length = int(self.headers.get('Content-Length', '0'))
            require(0 <= length <= 1_048_576, 'bounded consensus TLS request required')
            body = self.rfile.read(length)
            require(len(body) == length, 'complete consensus TLS request required')
            node = http.client.HTTPConnection(upstream.hostname, upstream.port, timeout=10)
            try:
                headers = {key: value for key, value in self.headers.items()
                           if key.lower() not in ('host', 'connection', 'content-length')}
                headers['Connection'] = 'close'
                node.request(self.command, self.path, body=body, headers=headers)
                response = node.getresponse()
                result = response.read(1_048_577)
                require(len(result) <= 1_048_576, 'bounded consensus TLS response required')
                save(Path(path).parent / ('consensus-http-' + uuid.uuid4().hex + '.json'),
                     {'method': self.command, 'path': self.path, 'request': body.hex(),
                      'status': response.status, 'headers': response.getheaders(),
                      'response': result.hex()})
                self.send_response_only(response.status)
                for key, value in response.getheaders():
                    if key.lower() not in ('connection', 'transfer-encoding', 'content-length'):
                        self.send_header(key, value)
                self.send_header('Content-Length', str(len(result)))
                self.send_header('Connection', 'close')
                self.end_headers()
                self.wfile.write(result)
                self.close_connection = True
            finally:
                node.close()

        do_GET = forward
        do_POST = forward

    with ThreadingHTTPServer(('127.0.0.1', config['port']), Tunnel) as server:
        print('consensus TLS tunnel listening', flush=True)
        server.serve_forever()


def worker(directory):
    for name in ('net', 'pid', 'mnt'):
        require(os.readlink('/proc/self/ns/' + name) != os.environ['PAXEER_X_PARENT_' + name.upper()], 'isolated native room namespace required')
    subprocess.run(['ip', 'link', 'set', 'lo', 'up'], check=True)
    build_directory, manifest = built_artifacts()
    driver = InferenceRoomDriver(directory, configuration(directory, build_directory, manifest),
                                 build_directory, manifest)
    try:
        driver.run()
    finally:
        driver.stop()
    print('PAXEER_X_GATE tests=2 skipped=0', flush=True)


def proof():
    evidence = envelope.private_dir(os.environ.get('PAXEER_X_EVIDENCE_DIR'), 'PAXEER_X_EVIDENCE_DIR')
    directory = evidence / ('inference-room-' + uuid.uuid4().hex)
    directory.mkdir(mode=0o700)
    revision = envelope.capture(['git', 'rev-parse', 'HEAD'])
    save(directory / 'invocation.json', {'schema': SCHEMA, 'task': TASK, 'revision': revision,
                                       'command': 'python3 tools/qualification/paxeer-x/inference_market_room.py --proof room',
                                       'evidence': str(directory)})
    env = dict(os.environ)
    for name in ('net', 'pid', 'mnt'):
        env['PAXEER_X_PARENT_' + name.upper()] = os.readlink('/proc/self/ns/' + name)
    argv = ['unshare', '--mount', '--net', '--pid', '--fork', '--kill-child=KILL', '--mount-proc',
            '--propagation', 'private', sys.executable, str(Path(__file__).resolve()), '--worker', str(directory)]
    print('INFERENCE_ROOM_EVIDENCE ' + str(directory), flush=True)
    return envelope.run_participant(argv, env, directory, revision, directory, timeout=900, log_name='native.log')


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(allow_abbrev=False)
    modes = parser.add_mutually_exclusive_group(required=True)
    modes.add_argument('--begin-build', action='store_true')
    modes.add_argument('--build', action='store_true')
    modes.add_argument('--proof', choices=('room',))
    modes.add_argument('--worker', help=argparse.SUPPRESS)
    modes.add_argument('--asker', help=argparse.SUPPRESS)
    modes.add_argument('--tls-tunnel', help=argparse.SUPPRESS)
    modes.add_argument('--identity-issuer', help=argparse.SUPPRESS)
    args = parser.parse_args()
    if args.begin_build:
        begin_build()
    elif args.build:
        build()
    elif args.worker:
        worker(args.worker)
    elif args.asker:
        asker(args.asker)
    elif args.tls_tunnel:
        tls_tunnel(args.tls_tunnel)
    elif args.identity_issuer:
        identity_issuer(args.identity_issuer)
    else:
        return proof()
    return 0


if __name__ == '__main__':
    raise SystemExit(main())

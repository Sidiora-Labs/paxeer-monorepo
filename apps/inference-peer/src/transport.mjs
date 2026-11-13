import { decodeFrame, encodeFrame, MAX_FRAME_BYTES } from "./wire.mjs";
import {
  decodeSignalFrame,
  encodeSignalFrame,
  MAX_SIGNAL_FRAME_BYTES,
  signalBytes,
  signalHex,
  validateSignalFrame,
} from "./signal.mjs";
import { signHello } from "./identity.mjs";
import { dealDigest } from "./deal.mjs";
import { lapCountersignDigest } from "./laps.mjs";
import { canaryAnswer, CanaryTracker } from "./canary.mjs";

function emit(target, type, detail) {
  target.dispatchEvent(new CustomEvent(type, { detail }));
}

function requireText(value, name) {
  if (typeof value !== "string" || !value.length)
    throw new TypeError(`${name} is required`);
}

export class SignalClient extends EventTarget {
  #socket;
  #resolve;
  #reject;
  #timer;
  #queue = Promise.resolve();
  #state = "idle";
  #deal;
  #digest;
  #lap;
  #canaries;

  constructor(
    url,
    {
      roomId,
      identity,
      manifest,
      WebSocket = globalThis.WebSocket,
      timeoutMs = 10000,
      canaryTracker,
    },
  ) {
    super();
    const parsed = new URL(url);
    if (!["ws:", "wss:"].includes(parsed.protocol))
      throw new TypeError("Expected a WebSocket URL");
    if (
      parsed.pathname !== "/" ||
      parsed.search ||
      parsed.hash ||
      parsed.username ||
      parsed.password
    )
      throw new TypeError("Signalling request target must be /");
    this.roomId = signalHex(roomId);
    if (typeof identity?.sign !== "function")
      throw new TypeError("Identity signer is required");
    signalBytes(identity.peerKey);
    if (
      !(manifest instanceof Uint8Array) ||
      manifest[0] !== 0 ||
      manifest[1] !== 2
    )
      throw new TypeError("A v2 manifest is required");
    if (typeof WebSocket !== "function")
      throw new Error("WebSocket API is unavailable");
    if (!Number.isFinite(timeoutMs) || timeoutMs <= 0)
      throw new RangeError("Invalid timeoutMs");
    if (canaryTracker !== undefined) {
      if (
        !(canaryTracker instanceof CanaryTracker) ||
        canaryTracker.peerKey !== signalHex(identity.peerKey)
      )
        throw new TypeError(
          "Canary tracker must belong to the signalling identity",
        );
      this.#canaries = canaryTracker;
    }
    this.url = parsed.href;
    this.identity = identity;
    this.manifest = manifest.slice();
    this.WebSocket = WebSocket;
    this.timeoutMs = timeoutMs;
  }
  get state() {
    return this.#state;
  }

  connect() {
    if (this.#state !== "idle")
      return Promise.reject(new Error("Client already started"));
    this.#state = "connecting";
    const ready = new Promise((resolve, reject) => {
      this.#resolve = resolve;
      this.#reject = reject;
    });
    try {
      const socket = (this.#socket = new this.WebSocket(this.url));
      this.#timer = setTimeout(
        () => this.#fail(new Error("Room offer timed out")),
        this.timeoutMs,
      );
      socket.addEventListener("open", () => {
        if (this.#state === "connecting") this.#state = "challenge";
      });
      socket.addEventListener("message", (event) => {
        this.#queue = this.#queue
          .then(() => this.#receive(event.data))
          .catch((error) => this.#fail(error));
      });
      socket.addEventListener("error", () =>
        this.#fail(
          new Error(
            "Signalling WebSocket closed after transport error",
          ),
        ),
      );
      socket.addEventListener("close", () => {
        if (this.#state !== "closed")
          this.#fail(new Error("Signalling WebSocket closed"));
      });
    } catch (error) {
      this.#fail(error);
    }
    return ready;
  }
  #write(frame) {
    if (this.#socket?.readyState !== 1)
      throw new Error("Signalling socket is not open");
    const text = encodeSignalFrame(frame);
    if (
      this.#socket.bufferedAmount +
        new TextEncoder().encode(text).length >
      MAX_SIGNAL_FRAME_BYTES * 4
    )
      throw new Error("Signalling backpressure limit reached");
    this.#socket.send(text);
  }
  async #receive(data) {
    if (this.#state === "closed") return;
    const frame = decodeSignalFrame(data);
    if (frame.type === "error") {
      const error = new Error(`Signalling host error: ${frame.code}`);
      error.code = frame.code;
      throw error;
    }
    if (this.#state === "challenge") {
      if (frame.type !== "challenge")
        throw new Error("Expected host challenge before hello");
      const hello = await signHello(this.identity, {
        roomId: this.roomId,
        nonce: frame.nonce,
        manifest: this.manifest,
      });
      if (this.#state === "closed") return;
      this.#write(hello);
      this.#state = "hello";
      return;
    }
    if (this.#state === "hello") {
      if (frame.type !== "offer")
        throw new Error("Expected room offer after hello");
      if (signalHex(frame.offer.room_id) !== this.roomId)
        throw new Error("Wrong signalling room");
      this.#state = "offered";
      clearTimeout(this.#timer);
      this.#resolve(frame);
    } else if (frame.type === "deal") {
      if (this.#state !== "offered" && this.#state !== "ready")
        throw new Error("Unexpected deal");
      if (signalHex(frame.deal.room_id) !== this.roomId)
        throw new Error("Wrong signalling room");
      this.#canaries?.close();
      this.#canaries = new CanaryTracker({
        identity: this.identity,
        deal: frame.deal,
      });
      this.#deal = structuredClone(frame.deal);
      this.#lap = undefined;
      this.#digest = Array.from(await dealDigest(this.#deal));
      if (this.#state === "closed") return;
      this.#state = "dealt";
    } else if (frame.type === "deal_live") {
      if (
        this.#state !== "acknowledged" ||
        signalHex(frame.deal_id) !== signalHex(this.#deal.deal_id)
      )
        throw new Error("No acknowledged deal matches deal_live");
      this.#state = "ready";
    } else if (frame.type === "canary_request") {
      const result =
        frame.plan.peer_key !== null &&
        signalHex(frame.plan.peer_key) !==
          signalHex(this.identity.peerKey)
          ? { type: "refusal", reason: "peer_mismatch" }
          : this.#canaries
            ? await canaryAnswer(frame.plan, this.#canaries)
            : { type: "refusal", reason: "canary_unavailable" };
      if (this.#state === "closed") return;
      if (result.type === "refusal") {
        emit(this, "canary-refusal", result);
      } else {
        this.#write({ type: "canary_answer", answer: result });
        emit(this, "canary", result);
      }
    } else if (frame.type === "lap_receipt") {
      if (
        this.#state !== "ready" ||
        signalHex(frame.lap.deal_digest) !== signalHex(this.#digest) ||
        signalHex(frame.lap.receipt.deal_id) !==
          signalHex(this.#deal.deal_id)
      )
        throw new Error("No live deal matches lap_receipt");
      this.#lap = {
        index: BigInt(frame.lap.receipt.lap_index),
        digest: signalHex(await lapCountersignDigest(frame.lap)),
      };
      if (this.#state === "closed") return;
    } else if (frame.type === "offer")
      throw new Error("Duplicate room offer");
    else if (
      !["description", "candidate"].includes(frame.type) ||
      !["offered", "dealt", "acknowledged", "ready"].includes(
        this.#state,
      )
    )
      throw new Error("Unexpected signalling frame");
    emit(this, "frame", frame);
  }
  // The runtime supplies actual logits after processing ordinary activations.
  recordCanaryLogits(record) {
    if (this.#state === "closed" || !this.#canaries)
      throw new Error("No active canary tracker");
    this.#canaries.recordLogits(record);
  }
  acknowledgeDeal(ack) {
    if (
      this.#state !== "dealt" ||
      signalHex(ack?.deal_id) !== signalHex(this.#deal.deal_id)
    )
      throw new Error("No matching deal to acknowledge");
    if (signalHex(ack.digest) !== signalHex(this.#digest))
      throw new Error("Acknowledgement digest differs from deal");
    this.#write(ack);
    this.#state = "acknowledged";
  }
  countersignLap(ack) {
    validateSignalFrame(ack, "client");
    if (
      ack.type !== "lap_countersign" ||
      this.#state !== "ready" ||
      !this.#lap ||
      signalHex(ack.deal_id) !== signalHex(this.#deal.deal_id) ||
      BigInt(ack.lap_index) !== this.#lap.index ||
      signalHex(ack.digest) !== this.#lap.digest
    )
      throw new Error("No matching lap to countersign");
    this.#write(ack);
  }
  relay(to, relay) {
    if (
      !["offered", "dealt", "acknowledged", "ready"].includes(
        this.#state,
      )
    )
      throw new Error("Room offer is required");
    validateRelay(relay);
    if (relay.description)
      this.#write({
        type: "description",
        to: signalBytes(to),
        description: relay.description,
      });
    else {
      const {
        candidate,
        sdpMid = null,
        sdpMLineIndex = null,
      } = relay.candidate;
      this.#write({
        type: "candidate",
        to: signalBytes(to),
        candidate: {
          candidate,
          sdp_mid: sdpMid,
          sdp_m_line_index: sdpMLineIndex,
        },
      });
    }
  }
  #fail(error) {
    if (this.#state === "closed") return;
    this.#reject?.(error);
    this.close();
    emit(this, "error", error);
  }
  close() {
    if (this.#state === "closed") return;
    this.#state = "closed";
    clearTimeout(this.#timer);
    this.#canaries?.close();
    this.#reject?.(new Error("Signalling client closed"));
    this.#socket?.close();
    emit(this, "close");
  }
}

function validateRelay(relay) {
  if (!relay || typeof relay !== "object")
    throw new TypeError("Invalid WebRTC relay");
  if (relay.description && !relay.candidate) {
    if (!["offer", "answer"].includes(relay.description.type))
      throw new TypeError("Invalid SDP type");
    requireText(relay.description.sdp, "SDP");
  } else if (relay.candidate && !relay.description) {
    requireText(relay.candidate.candidate, "ICE candidate");
    if (
      relay.candidate.sdpMid !== null &&
      relay.candidate.sdpMid !== undefined
    )
      requireText(relay.candidate.sdpMid, "ICE mid");
  } else
    throw new TypeError(
      "Relay must carry one description or candidate",
    );
}

export class PeerMesh extends EventTarget {
  #peers = new Map();
  #closed = false;
  #listener;
  #signalClosed;
  #native;

  static async create(signal, options = {}) {
    const mode = options.mode ?? "browser";
    if (!["browser", "native"].includes(mode))
      throw new TypeError("Unknown mesh mode");
    const native =
      mode === "native" ? await import("node-datachannel") : undefined;
    return new PeerMesh(signal, { ...options, mode }, native);
  }

  constructor(
    signal,
    {
      peerKey = signal.identity.peerKey,
      mode = "browser",
      rtcConfig = {},
    } = {},
    native,
  ) {
    super();
    requireText(peerKey, "peerKey");
    if (peerKey !== signal.identity.peerKey)
      throw new Error("Mesh identity differs from hello");
    if (!["browser", "native"].includes(mode))
      throw new TypeError("Unknown mesh mode");
    if (
      mode === "browser" &&
      typeof globalThis.RTCPeerConnection !== "function"
    ) {
      throw new Error("Browser RTCPeerConnection API is unavailable");
    }
    if (mode === "native" && !native)
      throw new Error("Use PeerMesh.create for native mode");
    this.signal = signal;
    this.peerKey = peerKey;
    this.mode = mode;
    this.rtcConfig = rtcConfig;
    this.#native = native;
    this.#listener = ({ detail: frame }) => {
      if (!["description", "candidate"].includes(frame.type)) return;
      const from = signalHex(frame.from);
      const relay =
        frame.type === "description"
          ? { description: frame.description }
          : {
              candidate: {
                candidate: frame.candidate.candidate,
                sdpMid: frame.candidate.sdp_mid,
                sdpMLineIndex: frame.candidate.sdp_m_line_index,
              },
            };
      const peer = this.#peers.get(from);
      if (!peer) {
        this.#fail(new Error("Relay from outside the deal"));
        return;
      }
      peer.queue = peer.queue
        .then(() => this.#receiveRelay(peer, relay))
        .catch((error) => this.#fail(error));
    };
    this.#signalClosed = () => this.close();
    signal.addEventListener("frame", this.#listener);
    signal.addEventListener("close", this.#signalClosed);
  }

  // Call after validating the host's chain-anchored offer and accepting the deal.
  async join(deal) {
    if (this.#closed || this.#peers.size)
      throw new Error("Mesh already joined or closed");
    requireText(deal?.dealId, "dealId");
    if (
      !Array.isArray(deal.peers) ||
      deal.peers.length < 2 ||
      deal.peers.length > 64 ||
      new Set(deal.peers).size !== deal.peers.length ||
      !deal.peers.includes(this.peerKey)
    ) {
      throw new TypeError(
        "Deal must name 2 to 64 distinct peers including this peer",
      );
    }
    for (const key of deal.peers) requireText(key, "Deal peer key");
    if (!["dealt", "acknowledged", "ready"].includes(this.signal.state))
      throw new Error("A received deal is required");
    try {
      // Allocate every connection before descriptions can arrive for any of them.
      for (const key of deal.peers) {
        if (key === this.peerKey) continue;
        this.#addPeer(key);
      }
      for (const peer of this.#peers.values()) {
        if (this.peerKey > peer.key) continue;
        for (const label of ["control", "activations"]) {
          const config =
            this.mode === "native"
              ? {
                  unordered: label === "activations",
                  protocol: "paxeer-room-v1",
                }
              : {
                  ordered: label === "control",
                  protocol: "paxeer-room-v1",
                };
          this.#attach(peer, peer.pc.createDataChannel(label, config));
        }
        if (this.mode === "native")
          peer.pc.setLocalDescription("offer");
        else {
          await peer.pc.setLocalDescription(
            await peer.pc.createOffer(),
          );
          this.#relay(peer, {
            description: peer.pc.localDescription.toJSON(),
          });
        }
      }
    } catch (error) {
      this.close();
      throw error;
    }
  }

  #addPeer(key) {
    const pc =
      this.mode === "native"
        ? new this.#native.PeerConnection(key, {
            iceServers: [],
            ...this.rtcConfig,
            maxMessageSize: MAX_FRAME_BYTES,
            disableAutoNegotiation: true,
          })
        : new globalThis.RTCPeerConnection(this.rtcConfig);
    const peer = {
      key,
      pc,
      channels: new Map(),
      queue: Promise.resolve(),
      candidates: [],
      remote: false,
      sequence: 0,
      received: -1,
    };
    this.#peers.set(key, peer);
    const attach = (channel) => {
      try {
        this.#attach(peer, channel);
      } catch (error) {
        channel.close();
        this.#fail(error);
      }
    };
    const stateChanged = (state) => {
      if (state === "failed" || state === "closed")
        this.#fail(new Error(`Peer ${key} ${state}`));
    };
    if (this.mode === "native") {
      pc.onLocalDescription((sdp, type) =>
        this.#relay(peer, { description: { sdp, type } }),
      );
      pc.onLocalCandidate((candidate, sdpMid) => {
        if (candidate)
          this.#relay(peer, { candidate: { candidate, sdpMid } });
      });
      pc.onDataChannel(attach);
      pc.onStateChange(stateChanged);
    } else {
      pc.addEventListener("icecandidate", ({ candidate }) => {
        if (candidate?.candidate)
          this.#relay(peer, { candidate: candidate.toJSON() });
      });
      pc.addEventListener("datachannel", ({ channel }) =>
        attach(channel),
      );
      pc.addEventListener("connectionstatechange", () =>
        stateChanged(pc.connectionState),
      );
    }
  }

  #relay(peer, relay) {
    if (this.#closed) return;
    try {
      this.signal.relay(peer.key, relay);
    } catch (error) {
      this.#fail(error);
    }
  }

  async #receiveRelay(peer, relay) {
    if (this.#closed) return;
    validateRelay(relay);
    if (relay.description) {
      const { type, sdp } = relay.description;
      if ((type === "offer") !== peer.key < this.peerKey)
        throw new Error("Unexpected SDP initiator");
      if (this.mode === "native")
        peer.pc.setRemoteDescription(sdp, type);
      else await peer.pc.setRemoteDescription(relay.description);
      peer.remote = true;
      for (const candidate of peer.candidates.splice(0))
        await this.#candidate(peer, candidate);
      if (type === "offer") {
        if (this.mode === "native")
          peer.pc.setLocalDescription("answer");
        else {
          await peer.pc.setLocalDescription(
            await peer.pc.createAnswer(),
          );
          this.#relay(peer, {
            description: peer.pc.localDescription.toJSON(),
          });
        }
      }
    } else if (!peer.remote) {
      if (peer.candidates.length >= 256)
        throw new Error("Too many pending ICE candidates");
      peer.candidates.push(relay.candidate);
    } else await this.#candidate(peer, relay.candidate);
  }

  async #candidate(peer, candidate) {
    if (this.mode === "native")
      peer.pc.addRemoteCandidate(candidate.candidate, candidate.sdpMid);
    else await peer.pc.addIceCandidate(candidate);
  }

  #attach(peer, channel) {
    if (this.#closed) {
      channel.close();
      return;
    }
    const native = this.mode === "native";
    const label = native ? channel.getLabel() : channel.label;
    if (
      !["control", "activations"].includes(label) ||
      peer.channels.has(label)
    ) {
      throw new Error("Unexpected or duplicate data channel");
    }
    if (native && channel.getProtocol() !== "paxeer-room-v1")
      throw new Error("Invalid data channel protocol");
    if (
      !native &&
      (channel.protocol !== "paxeer-room-v1" ||
        channel.ordered !== (label === "control") ||
        channel.maxRetransmits !== null ||
        channel.maxPacketLifeTime !== null)
    )
      throw new Error("Invalid data channel reliability");
    peer.channels.set(label, channel);
    const open = () => {
      if (!this.#closed)
        emit(this, "open", { peerKey: peer.key, channel: label });
    };
    const receive = (data) => {
      try {
        if (this.#closed) return;
        const frame = decodeFrame(data);
        if (frame.roomId !== this.signal.roomId)
          throw new Error("Wrong mesh room");
        if ((frame.kind === "activation") !== (label === "activations"))
          throw new Error("Frame on wrong channel");
        if (["hello", "offer", "deal", "deal-ack"].includes(frame.kind))
          throw new Error("Signalling frame on mesh");
        // Activations are unordered; their sequence is consumed by the layer scheduler.
        if (label === "control") {
          if (frame.sequence <= peer.received)
            throw new Error("Replayed control sequence");
          peer.received = frame.sequence;
        }
        emit(this, "frame", { peerKey: peer.key, frame });
        if (frame.kind === "bye") this.close();
      } catch (error) {
        this.#fail(error);
      }
    };
    const closed = () =>
      this.#fail(new Error(`Peer ${peer.key} channel closed`));
    if (native) {
      channel.onOpen(open);
      channel.onMessage(receive);
      channel.onError((message) => this.#fail(new Error(message)));
      channel.onClosed(closed);
    } else {
      channel.binaryType = "arraybuffer";
      channel.addEventListener("open", open);
      channel.addEventListener("message", ({ data }) => receive(data));
      channel.addEventListener("error", () =>
        this.#fail(new Error("Data channel error")),
      );
      channel.addEventListener("close", closed);
    }
  }

  send(peerKey, kind, payload) {
    if (this.#closed) throw new Error("Mesh is closed");
    if (["hello", "offer", "deal", "deal-ack"].includes(kind))
      throw new Error("Use signalling for this frame");
    const peer = this.#peers.get(peerKey);
    const channel = peer?.channels.get(
      kind === "activation" ? "activations" : "control",
    );
    const native = this.mode === "native";
    if (
      !channel ||
      !(native ? channel.isOpen() : channel.readyState === "open")
    ) {
      throw new Error("Peer data channel is not open");
    }
    const frame = encodeFrame({
      kind,
      roomId: this.signal.roomId,
      sequence: peer.sequence,
      payload,
    });
    const buffered = native
      ? channel.bufferedAmount()
      : channel.bufferedAmount;
    if (buffered + frame.byteLength > MAX_FRAME_BYTES * 4)
      throw new Error("Mesh backpressure limit reached");
    if (native) {
      if (frame.byteLength > channel.maxMessageSize())
        throw new RangeError("Frame exceeds negotiated message size");
      if (!channel.sendMessageBinary(frame))
        throw new Error("Data channel send refused");
    } else {
      if (
        peer.pc.sctp?.maxMessageSize &&
        frame.byteLength > peer.pc.sctp.maxMessageSize
      ) {
        throw new RangeError("Frame exceeds negotiated message size");
      }
      channel.send(frame);
    }
    peer.sequence++;
  }

  #fail(error) {
    if (this.#closed) return;
    this.close();
    emit(this, "error", error);
  }

  close() {
    if (this.#closed) return;
    this.#closed = true;
    this.signal.removeEventListener("frame", this.#listener);
    this.signal.removeEventListener("close", this.#signalClosed);
    for (const peer of this.#peers.values()) {
      for (const channel of peer.channels.values()) channel.close();
      peer.pc.close();
    }
    this.#peers.clear();
    emit(this, "close");
  }
}

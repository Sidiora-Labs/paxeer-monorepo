import { decodeFrame, encodeFrame, MAX_FRAME_BYTES } from "./wire.mjs";

function emit(target, type, detail) {
  target.dispatchEvent(new CustomEvent(type, { detail }));
}

function requireText(value, name) {
  if (typeof value !== "string" || !value.length)
    throw new TypeError(`${name} is required`);
}

export class SignalClient extends EventTarget {
  #socket;
  #sequence = 0;
  #received = -1;
  #resolve;
  #reject;
  #timer;
  #queue = Promise.resolve();
  #state = "idle";
  #deal;

  constructor(
    url,
    { roomId, hello, WebSocket = globalThis.WebSocket, timeoutMs = 10000 },
  ) {
    super();
    const parsed = new URL(url);
    if (!["ws:", "wss:"].includes(parsed.protocol))
      throw new TypeError("Expected a WebSocket URL");
    requireText(roomId, "roomId");
    for (const name of ["peerKey", "signature", "nonce"])
      requireText(hello?.[name], name);
    if (!hello.manifest || hello.manifest.version !== 2)
      throw new TypeError("A v2 manifest is required");
    if (typeof WebSocket !== "function")
      throw new Error("WebSocket API is unavailable");
    if (!Number.isFinite(timeoutMs) || timeoutMs <= 0)
      throw new RangeError("Invalid timeoutMs");
    this.url = parsed.href;
    this.roomId = roomId;
    this.hello = structuredClone(hello);
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
      socket.binaryType = "arraybuffer";
      this.#timer = setTimeout(
        () => this.#fail(new Error("Room offer timed out")),
        this.timeoutMs,
      );
      socket.addEventListener("open", () => {
        try {
          this.#write("hello", this.hello);
          this.#state = "hello";
        } catch (error) {
          this.#fail(error);
        }
      });
      socket.addEventListener("message", (event) => {
        this.#queue = this.#queue
          .then(() => this.#receive(event.data))
          .catch((error) => this.#fail(error));
      });
      socket.addEventListener("error", () =>
        this.#fail(
          new Error("Signalling WebSocket closed after transport error"),
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

  #write(kind, payload) {
    if (this.#sequence > 0xffffffff)
      throw new RangeError("Signalling sequence exhausted");
    if (this.#socket?.readyState !== 1)
      throw new Error("Signalling socket is not open");
    const frame = encodeFrame({
      kind,
      roomId: this.roomId,
      sequence: this.#sequence,
      payload,
    });
    if (this.#socket.bufferedAmount + frame.byteLength > MAX_FRAME_BYTES * 4) {
      throw new Error("Signalling backpressure limit reached");
    }
    this.#socket.send(frame);
    this.#sequence++;
  }

  async #receive(data) {
    if (this.#state === "closed") return;
    const frame = decodeFrame(data);
    if (frame.roomId !== this.roomId) throw new Error("Wrong signalling room");
    if (frame.sequence <= this.#received)
      throw new Error("Replayed signalling sequence");
    this.#received = frame.sequence;
    const relay = frame.kind === "offer" && frame.payload.relay !== undefined;
    if (this.#state === "hello") {
      if (frame.kind !== "offer" || relay)
        throw new Error("Expected room offer after hello");
      this.#state = "offered";
      clearTimeout(this.#timer);
      this.#resolve(frame);
    } else if (frame.kind === "deal") {
      if (this.#state !== "offered" && this.#state !== "ready")
        throw new Error("Unexpected deal");
      requireText(frame.payload.dealId, "dealId");
      this.#deal = frame.payload.dealId;
      this.#state = "dealt";
    } else if (frame.kind === "offer" && !relay) {
      throw new Error("Duplicate room offer");
    } else if (["hello", "deal-ack", "activation"].includes(frame.kind)) {
      throw new Error("Unexpected signalling frame");
    }
    emit(this, "frame", frame);
    if (frame.kind === "bye") this.close();
  }

  acknowledgeDeal(dealId) {
    if (this.#state !== "dealt" || this.#deal !== dealId)
      throw new Error("No matching deal to acknowledge");
    this.#write("deal-ack", { dealId });
    this.#state = "ready";
  }

  relay(to, relay) {
    if (!["offered", "dealt", "ready"].includes(this.#state))
      throw new Error("Room offer is required");
    requireText(to, "Relay recipient");
    validateRelay(relay);
    this.#write("offer", { to, relay });
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
    requireText(relay.candidate.sdpMid, "ICE mid");
  } else throw new TypeError("Relay must carry one description or candidate");
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
    { peerKey = signal.hello.peerKey, mode = "browser", rtcConfig = {} } = {},
    native,
  ) {
    super();
    requireText(peerKey, "peerKey");
    if (peerKey !== signal.hello.peerKey)
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
      if (frame.kind !== "offer" || !frame.payload.relay) return;
      const { from, to, relay } = frame.payload;
      const peer = this.#peers.get(from);
      if (to !== this.peerKey || !peer) {
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
    if (this.signal.state !== "dealt")
      throw new Error("A received deal is required");
    try {
      // Allocate every connection before descriptions can arrive for any of them.
      for (const key of deal.peers) {
        if (key === this.peerKey) continue;
        this.#addPeer(key);
      }
      this.signal.acknowledgeDeal(deal.dealId);
      for (const peer of this.#peers.values()) {
        if (this.peerKey > peer.key) continue;
        for (const label of ["control", "activations"]) {
          const config =
            this.mode === "native"
              ? {
                  unordered: label === "activations",
                  protocol: "paxeer-room-v1",
                }
              : { ordered: label === "control", protocol: "paxeer-room-v1" };
          this.#attach(peer, peer.pc.createDataChannel(label, config));
        }
        if (this.mode === "native") peer.pc.setLocalDescription("offer");
        else {
          await peer.pc.setLocalDescription(await peer.pc.createOffer());
          this.#relay(peer, { description: peer.pc.localDescription.toJSON() });
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
        if (candidate) this.#relay(peer, { candidate: { candidate, sdpMid } });
      });
      pc.onDataChannel(attach);
      pc.onStateChange(stateChanged);
    } else {
      pc.addEventListener("icecandidate", ({ candidate }) => {
        if (candidate?.candidate)
          this.#relay(peer, { candidate: candidate.toJSON() });
      });
      pc.addEventListener("datachannel", ({ channel }) => attach(channel));
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
      if (this.mode === "native") peer.pc.setRemoteDescription(sdp, type);
      else await peer.pc.setRemoteDescription(relay.description);
      peer.remote = true;
      for (const candidate of peer.candidates.splice(0))
        await this.#candidate(peer, candidate);
      if (type === "offer") {
        if (this.mode === "native") peer.pc.setLocalDescription("answer");
        else {
          await peer.pc.setLocalDescription(await peer.pc.createAnswer());
          this.#relay(peer, { description: peer.pc.localDescription.toJSON() });
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
    const buffered = native ? channel.bufferedAmount() : channel.bufferedAmount;
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

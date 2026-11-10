import { measureManifest, PeerIdentity } from "./identity.mjs";
import { SignalClient } from "./transport.mjs";
import { signalHex, validateSignalFrame } from "./signal.mjs";

const unavailable = "Unavailable — host does not send this state";

function hex(bytes) {
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join(
    "",
  );
}

function manifestBytes(value) {
  const input = value.trim();
  if (!/^(?:[0-9a-fA-F]{2})+$/.test(input))
    throw new TypeError("Paste the participant manifest as hexadecimal bytes");
  return Uint8Array.from(input.match(/../g), (byte) => parseInt(byte, 16));
}

function display(value) {
  return JSON.stringify(
    value,
    (_, item) => (typeof item === "bigint" ? item.toString() : item),
    2,
  );
}

function element(document, id) {
  const node = document.getElementById(id);
  if (!node) throw new Error(`Peer page is missing #${id}`);
  return node;
}

// Only host signalling members are displayed. No status/income is inferred.
export function renderStatus(document, frame, peerKey) {
  validateSignalFrame(frame);
  let layers;
  if (frame.type === "offer") {
    layers = frame.offer.layer_plan;
    element(document, "offer").textContent = display(frame.offer);
    element(document, "connection").textContent = "Joined room — awaiting deal";
  } else if (frame.type === "deal") {
    layers = frame.deal.plan.layers;
    element(document, "deal").textContent = signalHex(frame.deal.deal_id);
    element(document, "deal-details").textContent = display(frame.deal);
    element(document, "connection").textContent =
      "Deal received — awaiting acceptance";
  } else if (frame.type === "deal_live") {
    element(document, "deal").textContent = signalHex(frame.deal_id);
    element(document, "connection").textContent = "Host reports deal live";
  } else if (frame.type === "error") {
    element(document, "error").textContent = frame.code;
  }
  if (layers) {
    const assignment = layers.find(
      (split) => signalHex(split.peer_key) === peerKey,
    );
    if (
      assignment &&
      (!Number.isSafeInteger(assignment.first_layer) ||
        !Number.isSafeInteger(assignment.end_layer) ||
        assignment.first_layer < 0 ||
        assignment.first_layer >= assignment.end_layer)
    )
      throw new TypeError("Invalid assigned layer range");
    element(document, "layers").textContent = assignment
      ? `[${assignment.first_layer}, ${assignment.end_layer})`
      : "No layers assigned to this peer";
  }
  for (const id of ["laps", "earnings", "receipts"])
    element(document, id).textContent = unavailable;
}

export class PeerPage {
  #identity;
  #manifest;
  #signal;
  #busy = false;
  #started = false;
  #disposed = false;
  #generation = 0;
  #listeners = [];

  constructor(
    document,
    { identityOptions = { mode: "browser" }, mode = "browser" } = {},
  ) {
    this.document = document;
    this.window = document.defaultView;
    this.identityOptions = identityOptions;
    this.mode = mode;
    if (!this.window)
      throw new TypeError("PeerPage requires a browser document");
  }

  #on(target, name, listener) {
    target.addEventListener(name, listener);
    this.#listeners.push(() => target.removeEventListener(name, listener));
  }

  #text(id, text) {
    element(this.document, id).textContent = text;
  }

  #controls() {
    const active = this.#signal !== undefined;
    element(this.document, "measure").disabled =
      !this.#identity || active || this.#busy || this.#disposed;
    element(this.document, "join").disabled =
      !this.#manifest || active || this.#busy || this.#disposed;
    element(this.document, "leave").disabled = !active;
    for (const id of ["manifest-v1", "region", "probe-url", "host", "room"])
      element(this.document, id).disabled =
        active || this.#busy || this.#disposed;
  }

  showError(error) {
    this.#text(
      "error",
      error instanceof Error ? (error.code ?? error.message) : String(error),
    );
  }

  async start() {
    if (this.#started || this.#disposed)
      throw new Error("Peer page already started or disposed");
    this.#started = true;
    this.#on(element(this.document, "join-form"), "submit", (event) => {
      event.preventDefault();
      this.join().catch((error) => this.showError(error));
    });
    this.#on(element(this.document, "measure"), "click", () => {
      this.measure().catch((error) => this.showError(error));
    });
    this.#on(element(this.document, "leave"), "click", () => this.leave());
    const invalidate = () => {
      this.#manifest = undefined;
      this.#text("manifest", "Not measured");
      this.#controls();
    };
    for (const id of ["manifest-v1", "region", "probe-url"])
      this.#on(element(this.document, id), "input", invalidate);
    this.#on(this.window, "pagehide", () => this.leave());
    this.#on(this.window, "beforeunload", () => this.leave());
    this.#on(this.window, "unload", () => this.leave());
    element(this.document, "probe-url").value = new URL(
      "/probe",
      this.window.location.href,
    ).href;
    try {
      this.#identity = await PeerIdentity.open(this.identityOptions);
      if (this.#disposed) return;
      this.#text("peer-key", this.#identity.peerKey);
      const gpu = this.window.navigator.gpu;
      if (!gpu) this.#text("adapter", "WebGPU unavailable in this browser");
      else {
        const adapter = await gpu.requestAdapter();
        if (this.#disposed) return;
        if (!adapter) throw new Error("WebGPU adapter is unavailable");
        const info = adapter.info;
        const name = [
          info?.vendor,
          info?.architecture,
          info?.device,
          info?.description,
        ]
          .filter(Boolean)
          .join(" · ");
        this.#text(
          "adapter",
          `${name || "WebGPU adapter"} — max buffer ${Math.floor(adapter.limits.maxBufferSize / 1048576)} MiB`,
        );
      }
      this.#text("connection", "Measure this device before joining a room");
    } catch (error) {
      this.#text("connection", "Device setup failed");
      this.showError(error);
      throw error;
    } finally {
      this.#controls();
    }
  }

  async measure() {
    if (!this.#identity || this.#signal || this.#busy || this.#disposed)
      throw new Error("Device measurement is not available now");
    this.#busy = true;
    this.#manifest = undefined;
    this.#text("error", "");
    this.#text("manifest", "Measuring…");
    this.#controls();
    try {
      const manifest = await measureManifest({
        manifestV1: manifestBytes(element(this.document, "manifest-v1").value),
        regionCode: element(this.document, "region").value.trim(),
        probeUrl: element(this.document, "probe-url").value.trim(),
        mode: this.mode,
      });
      if (this.#disposed) return;
      this.#manifest = manifest;
      const offset = manifest.length - 13;
      const view = new DataView(
        manifest.buffer,
        manifest.byteOffset,
        manifest.byteLength,
      );
      this.#text(
        "manifest",
        JSON.stringify(
          {
            version: 2,
            acceleratorMemoryMiB: view.getUint32(offset),
            systemMemoryMiB: view.getUint32(offset + 4),
            bandwidthClass: view.getUint8(offset + 8),
            regionCode: new TextDecoder().decode(
              manifest.subarray(offset + 9, offset + 11),
            ),
            attestationFamily: view.getUint8(offset + 11),
            runtimeKind: view.getUint8(offset + 12),
            bytes: hex(manifest),
          },
          null,
          2,
        ),
      );
      this.#text("connection", "Ready to join");
      return manifest.slice();
    } catch (error) {
      this.#text("manifest", "Measurement failed");
      this.showError(error);
      throw error;
    } finally {
      this.#busy = false;
      this.#controls();
    }
  }

  async join() {
    if (!this.#manifest || this.#signal || this.#busy || this.#disposed)
      throw new Error(
        "Measure this device before joining; leave any current room first",
      );
    const host = new URL(element(this.document, "host").value.trim());
    if (
      !["ws:", "wss:"].includes(host.protocol) ||
      host.username ||
      host.password ||
      host.hash
    )
      throw new TypeError(
        "Use a ws:// or wss:// signalling address without credentials or fragment",
      );
    const roomId = element(this.document, "room").value.trim();
    signalHex(roomId);
    this.#busy = true;
    const generation = ++this.#generation;
    this.#text("error", "");
    this.#text("connection", "Joining…");
    this.#controls();
    let signal;
    try {
      signal = new SignalClient(host.href, {
        roomId,
        identity: this.#identity,
        manifest: this.#manifest,
        WebSocket: this.window.WebSocket,
      });
      this.#signal = signal;
      for (const [id, value] of [
        ["offer", "Awaiting host"],
        ["deal", "Awaiting host"],
        ["deal-details", "Awaiting host"],
        ["layers", "Awaiting assignment"],
        ["laps", unavailable],
        ["earnings", unavailable],
        ["receipts", unavailable],
      ])
        this.#text(id, value);
      signal.addEventListener("frame", ({ detail: frame }) => {
        if (this.#signal !== signal) return;
        try {
          renderStatus(this.document, frame, this.#identity.peerKey);
        } catch (error) {
          this.showError(error);
          this.leave();
        }
      });
      signal.addEventListener("error", ({ detail: error }) => {
        if (generation !== this.#generation) return;
        if (error.code)
          renderStatus(
            this.document,
            { type: "error", code: error.code },
            this.#identity.peerKey,
          );
        else this.showError(error);
      });
      signal.addEventListener("close", () => {
        if (this.#signal !== signal) return;
        this.#signal = undefined;
        this.#text("connection", "Disconnected");
        this.#controls();
      });
      this.#controls();
      const offer = await signal.connect();
      return offer;
    } catch (error) {
      if (generation !== this.#generation) return;
      if (this.#signal === signal) this.leave();
      this.showError(error.code ?? error);
      throw error;
    } finally {
      this.#busy = false;
      this.#controls();
    }
  }

  leave() {
    this.#generation++;
    const signal = this.#signal;
    this.#signal = undefined;
    try {
      signal?.close();
    } catch (error) {
      this.showError(error);
    } finally {
      if (signal) this.#text("connection", "Left room");
      this.#controls();
    }
  }

  dispose() {
    this.#disposed = true;
    this.leave();
    for (const remove of this.#listeners.splice(0)) remove();
    this.#controls();
  }
}

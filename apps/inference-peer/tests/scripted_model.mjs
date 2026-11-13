const encoder = new TextEncoder();

function matrix(seed, width) {
  return Array.from({ length: width }, (_, row) =>
    Array.from(
      { length: width },
      (_, col) => Math.sin(seed + row * width + col) * 0.35,
    ),
  );
}

function multiply(weights, values) {
  return Float32Array.from(weights, (row) =>
    row.reduce((sum, value, i) => sum + value * values[i], 0),
  );
}

export function scriptedEmbedding(token, width = 4) {
  return Float32Array.from({ length: width }, (_, i) =>
    Math.sin((token + 1) * (i + 1)),
  );
}

export function scriptedToken(values, vocab = 19) {
  let token = 0,
    best = -Infinity;
  for (let i = 0; i < vocab; i++) {
    const score = values.reduce(
      (sum, value, j) => sum + value * Math.cos((i + 1) * (j + 1)),
      0,
    );
    if (score > best) {
      best = score;
      token = i;
    }
  }
  return token;
}

export class ScriptedModel {
  constructor(layers, width = 4) {
    this.layers = layers;
    this.width = width;
    this.liveCaches = new Set();
    this.droppedCaches = [];
  }

  createCache(id) {
    const cache = {
      id,
      position: 0,
      layers: new Map(
        this.layers.map((layer) => [layer.index, { keys: [], values: [] }]),
      ),
    };
    this.liveCaches.add(cache);
    return cache;
  }

  async runActivation(input, position, cache) {
    if (
      !this.liveCaches.has(cache) ||
      cache.position !== position ||
      input.length !== this.width
    )
      throw new Error("Invalid scripted cache or token position");
    let hidden = input;
    for (const layer of this.layers) {
      const history = cache.layers.get(layer.index);
      const query = multiply(layer.q, hidden);
      history.keys.push(multiply(layer.k, hidden));
      history.values.push(multiply(layer.v, hidden));
      const scores = history.keys.map(
        (key) =>
          key.reduce((sum, value, i) => sum + value * query[i], 0) /
          Math.sqrt(this.width),
      );
      const max = Math.max(...scores),
        weights = scores.map((score) => Math.exp(score - max));
      const total = weights.reduce((a, b) => a + b, 0);
      const context = Float32Array.from({ length: this.width }, (_, i) =>
        history.values.reduce(
          (sum, value, j) => sum + (weights[j] * value[i]) / total,
          0,
        ),
      );
      const projected = multiply(layer.o, context);
      hidden = Float32Array.from(hidden, (value, i) =>
        Math.tanh(value + projected[i] + layer.bias[i]),
      );
    }
    cache.position++;
    return hidden;
  }

  dropCache(cache) {
    for (const history of cache.layers.values()) {
      for (const values of [...history.keys, ...history.values]) values.fill(0);
      history.keys.length = 0;
      history.values.length = 0;
    }
    cache.layers.clear();
    cache.position = 0;
    this.liveCaches.delete(cache);
    this.droppedCaches.push(cache);
  }

  close() {
    for (const cache of this.liveCaches) this.dropCache(cache);
  }
}

export class ScriptedRuntime {
  models = [];
  async loadLayerRange({ manifest, shards, firstLayer, endLayer }) {
    if (manifest.format !== "scripted-attention-v1")
      throw new Error("Unsupported scripted model format");
    const layers = shards.map((shard) =>
      JSON.parse(new TextDecoder().decode(shard.bytes)),
    );
    if (
      layers.length !== endLayer - firstLayer ||
      layers.some((layer, i) => layer.index !== firstLayer + i)
    )
      throw new Error("Scripted shard range differs");
    const model = new ScriptedModel(layers, manifest.hidden_size);
    this.models.push(model);
    return model;
  }
}

export async function scriptedModelSource({ layerCount = 4, width = 4 } = {}) {
  const resources = new Map(),
    shards = [],
    layers = [];
  for (let i = 0; i < layerCount; i++) {
    const layer = {
      index: i,
      q: matrix(10 + i, width),
      k: matrix(30 + i, width),
      v: matrix(50 + i, width),
      o: matrix(70 + i, width),
      bias: Array.from({ length: width }, (_, j) => Math.cos(i + j) * 0.1),
    };
    layers.push(layer);
    const bytes = encoder.encode(JSON.stringify(layer)),
      path = `layer-${i}.json`;
    resources.set(`/${path}`, bytes);
    const sha256 = Array.from(
      new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)),
      (b) => b.toString(16).padStart(2, "0"),
    ).join("");
    shards.push({
      path,
      first_layer: i,
      end_layer: i + 1,
      size: bytes.length,
      sha256,
    });
  }
  const manifest = {
    version: 1,
    format: "scripted-attention-v1",
    layer_count: layerCount,
    hidden_size: width,
    shards,
  };
  const bytes = encoder.encode(JSON.stringify(manifest));
  resources.set("/manifest.json", bytes);
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", bytes));
  return {
    resources,
    manifest,
    digest,
    model: new ScriptedModel(layers, width),
  };
}

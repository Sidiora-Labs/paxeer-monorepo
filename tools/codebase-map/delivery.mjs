export const TASK_PAGE_SIZE = 160;
export const WAVE_PAGE_SIZE = 80;

export class DeliveryMap {
  constructor(tasks) {
    this.tasks = new Map();
    this.byWave = new Map();
    this.positions = new Map();
    this.dependents = new Map();
    for (const task of tasks) {
      this.tasks.set(task.id, task);
      if (!task.deliverable) continue;
      const peers = this.byWave.get(task.wave) ?? [];
      this.positions.set(task.id, peers.length);
      peers.push(task);
      this.byWave.set(task.wave, peers);
    }
    this.waves = [...this.byWave.keys()].sort((left, right) => left - right);
    this.waveEdges = new Map();
    for (const task of tasks) {
      for (const required of task.requires) {
        const dependents = this.dependents.get(required) ?? [];
        dependents.push(task);
        this.dependents.set(required, dependents);
        const prerequisite = this.tasks.get(required);
        if (!task.deliverable || !prerequisite?.deliverable || prerequisite.wave === task.wave) continue;
        const key = `${prerequisite.wave}:${task.wave}`;
        const edges = this.waveEdges.get(prerequisite.wave) ?? new Map();
        const edge = edges.get(key) ?? { source: `w:${prerequisite.wave}`, target: `w:${task.wave}`, total: 0, kind: "requires" };
        edge.total += 1;
        edges.set(key, edge);
        this.waveEdges.set(prerequisite.wave, edges);
      }
    }
  }

  scopeForTask(id) {
    const task = this.tasks.get(id);
    return task?.deliverable ? { wave: task.wave, page: Math.floor(this.positions.get(id) / TASK_PAGE_SIZE) } : {};
  }

  search(query) {
    const needle = query.trim().toLowerCase();
    if (needle.length < 2) return [];
    const matches = [];
    for (const task of this.tasks.values()) {
      const title = `${task.id} · ${task.title}`;
      const subtitle = `Wave ${task.wave ?? "group"} · ${task.status}`;
      const haystack = `${title} ${subtitle} ${task.symbols.join(" ")} ${task.touches.join(" ")}`.toLowerCase();
      const position = haystack.indexOf(needle);
      if (position < 0) continue;
      const score = (position === 0 ? 100 : 30) + (title.toLowerCase() === needle ? 200 : 0) + (title.toLowerCase().includes(needle) ? 50 : 0);
      matches.push({ type: "task", id: task.id, title, subtitle, score });
    }
    return matches;
  }

  view(scope = {}) {
    const expanded = scope.wave !== undefined && scope.wave !== null;
    const all = expanded ? (this.byWave.get(scope.wave) ?? []) : this.waves;
    const size = expanded ? TASK_PAGE_SIZE : WAVE_PAGE_SIZE;
    const pageCount = Math.max(1, Math.ceil(all.length / size));
    const page = Math.min(pageCount - 1, Math.max(0, Number.isInteger(scope.page) ? scope.page : 0));
    const slice = all.slice(page * size, (page + 1) * size);
    const items = expanded ? slice : slice.map((wave) => ({ wave, count: this.byWave.get(wave).length }));
    const ids = new Set(items.map((item) => expanded ? `t:${item.id}` : `w:${item.wave}`));
    const edges = [];
    for (const item of items) {
      if (expanded) {
        for (const required of item.requires) {
          if (ids.has(`t:${required}`)) edges.push({ source: `t:${required}`, target: `t:${item.id}`, total: 1, kind: "requires" });
        }
      } else {
        for (const edge of this.waveEdges.get(item.wave)?.values() ?? []) {
          if (ids.has(edge.target)) edges.push(edge);
        }
      }
    }
    return { expanded, items, edges, page, pageCount, total: all.length };
  }
}

export function deliveryLayout(nodes) {
  const columns = Math.max(1, Math.ceil(Math.sqrt(nodes.length)));
  const rows = Math.ceil(nodes.length / columns);
  return nodes.map((node, position) => ({
    ...node,
    initialX: ((position % columns) - (columns - 1) / 2) * 135,
    initialY: (Math.floor(position / columns) - (rows - 1) / 2) * 65,
  }));
}

export function deliveryGraph(delivery, scope, colors) {
  const view = delivery.view(scope);
  const nodes = view.expanded ? view.items.map((task) => ({
    id: `t:${task.id}`, entityType: "task", entityId: task.id, label: task.id,
    secondary: task.title, value: 1, color: colors[task.status] ?? colors.task,
  })) : view.items.map(({ wave, count }) => ({
    id: `w:${wave}`, entityType: "wave", entityId: wave, label: `Wave ${wave}`,
    secondary: `${count} tasks · open to expand`, value: count, color: colors.task,
  }));
  return { nodes: deliveryLayout(nodes), edges: view.edges, total: view.total, layout: "fixed" };
}

export async function loadSnapshot(url = "data/codebase-map.json") {
  const response = await fetch(url, { cache: "no-store" });
  if (!response.ok) throw new Error(`Map data returned HTTP ${response.status}. Run \`npm run generate\`.`);
  const snapshot = await response.json();
  if (snapshot.schemaVersion !== 1) throw new Error(`Unsupported map schema ${snapshot.schemaVersion}.`);
  return snapshot;
}

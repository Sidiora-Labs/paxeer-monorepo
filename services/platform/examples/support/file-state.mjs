import { channel } from "node:diagnostics_channel";
import { link, mkdir, open, readFile, unlink } from "node:fs/promises";
import { dirname, join } from "node:path";

const publication = channel("layerx.examples.state.publication");

async function syncParent(path) {
  const directory = await open(dirname(path), "r");
  try {
    await directory.sync();
  } finally {
    await directory.close();
  }
}

async function durableDirectory(path) {
  try {
    await mkdir(path, { mode: 0o700 });
  } catch (error) {
    if (error.code === "ENOENT") {
      await durableDirectory(dirname(path));
      return durableDirectory(path);
    }
    if (error.code !== "EEXIST") throw error;
  }
  await syncParent(path);
}

// Each immutable successor is the exclusive CAS slot for its predecessor.
// Readers also sync the directory: observing a winner must make it durable
// before acknowledging a retry after the publisher died before directory sync.
export class FileCompareAndSet {
  constructor(directory) {
    this.directory = directory;
  }

  path(key, version = 0) {
    if (typeof key !== "string" || !/^[A-Za-z0-9._-]{1,255}$/u.test(key)) {
      throw new Error("invalid_identifier");
    }
    if (!Number.isSafeInteger(version) || version < 0) throw new Error("invalid_state_version");
    return join(this.directory, `${key}.json${version === 0 ? "" : `.${version}`}`);
  }

  async read(key) {
    let stored;
    for (let version = 0; ; version += 1) {
      try {
        const value = JSON.parse(await readFile(this.path(key, version), "utf8"));
        if (value === null || typeof value !== "object" || Array.isArray(value)) {
          throw new Error("invalid_state_record");
        }
        stored = { version, value };
      } catch (error) {
        if (error.code !== "ENOENT") throw error;
        if (stored !== undefined) await syncParent(this.path(key));
        return stored;
      }
    }
  }

  async compareAndSet(key, expectedVersion, value, leaseUntilMs) {
    if (!Number.isSafeInteger(expectedVersion) || expectedVersion < -1) throw new Error("invalid_state_version");
    const path = this.path(key, expectedVersion + 1);
    const content = JSON.stringify(value);
    if (expectedVersion >= 0) await readFile(this.path(key, expectedVersion));
    await durableDirectory(this.directory);
    const temporary = `${path}.${process.pid}.${crypto.randomUUID()}.pending`;
    const file = await open(temporary, "wx", 0o600);
    let created = false;
    try {
      try {
        publication.publish({ stage: "created", path, temporary });
        await file.writeFile(content, "utf8");
        publication.publish({ stage: "written", path, temporary });
        await file.sync();
        publication.publish({ stage: "synced", path, temporary });
      } finally {
        await file.close();
      }
      if (leaseUntilMs !== undefined && leaseUntilMs <= Date.now()) return false;
      try {
        await link(temporary, path);
        created = true;
        publication.publish({ stage: "published", path, temporary });
      } catch (error) {
        if (error.code !== "EEXIST") throw error;
      }
      await syncParent(path);
      publication.publish({ stage: "durable", path, temporary });
      return created;
    } finally {
      await unlink(temporary);
    }
  }
}

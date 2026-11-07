import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createServer } from "node:http";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { build } from "../../../node_modules/esbuild/lib/main.js";
import { build as buildStyles } from "../../../apps/web/node_modules/vite/dist/node/index.js";
import tailwindcss from "../../../apps/web/node_modules/@tailwindcss/vite/dist/index.mjs";
import { executeSourceAction } from "../../../plugins/hyperfide-source-control/runtime.ts";
import sourceHost from "../../../plugins/environment-slices/host.ts";
import { requestSchema } from "../../../plugins/hyperfide-source-control/contract.ts";

const root = path.resolve(import.meta.dirname, "../../..");
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
export async function runSourceProductBrowser(f, binding, inspection) {
  const temp = await fs.mkdtemp(path.join(os.tmpdir(), "hf-source-browser-"));
  const output = path.join(
    root,
    inspection?.outputDirectory ?? "build/source-control/product/browser",
  );
  await fs.mkdir(output, { recursive: true });
  let server, chrome, socket, cdp;
  let loseCheckpoint = true;
  const diffRequests = [];
  const widths = [
    { name: "desktop", width: 1440, height: 1000 },
    { name: "tablet", width: 768, height: 1024 },
    { name: "mobile", width: 390, height: 844 },
  ];
  const screenshots = [];
  try {
    let base = f.base;
    const baselineContent = "candidate: browser baseline\n";
    if (!inspection) {
      base = await f.createRevision({
        text: baselineContent,
        parent_revision_ids: [],
      });
      const candidate = await f.createCandidate(base, {
        domain: await f.createDomain(base),
      });
      const qualified = await f.qualify(candidate);
      assert.equal(qualified.execution.observed.exit_code, 0);
      const accepted = await f.accept(candidate, qualified);
      assert.equal(accepted.revision_id, base.revision_id);
      assert.equal(accepted.root_tree_id, base.root_tree_id);
    }
    const entry = path.join(temp, "entry.tsx"),
      bundle = path.join(temp, "app.js");
    await fs.writeFile(
      entry,
      `import React from ${JSON.stringify(path.join(root, "plugins/hyperfide-source-control/node_modules/react/index.js"))};\nimport{createRoot}from ${JSON.stringify(path.join(root, "apps/web/node_modules/react-dom/client.js"))};\nimport plugin from ${JSON.stringify(path.join(root, "plugins/hyperfide-source-control/app.tsx"))};\nconst rpc={call:async(name,input)=>{const response=await fetch('/rpc/'+name,{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(input)});if(!response.ok)throw Error('RPC request failed');return response.json();}};globalThis.__hyperfidePluginRuntime={pluginSdkApp:{useRpc:()=>rpc}};plugin.setup({slots:{navPanel(panel){if(!/^[a-zA-Z0-9_-]+$/.test(panel.path))throw Error('Invalid panel route');const Panel=panel.component;createRoot(document.getElementById('root')).render(<Panel subPath=""/>);}}});`,
    );
    await build({
      entryPoints: [entry],
      outfile: bundle,
      bundle: true,
      format: "esm",
      platform: "browser",
      conditions: ["source"],
      jsx: "automatic",
      alias: {
        react: path.join(
          root,
          "plugins/hyperfide-source-control/node_modules/react",
        ),
      },
      tsconfig: path.join(
        root,
        "plugins/hyperfide-source-control/tsconfig.json",
      ),
      logLevel: "silent",
    });
    const stylesheet = path.join(temp, "product.css");
    await fs.writeFile(
      stylesheet,
      `@import ${JSON.stringify(path.join(root, "apps/web/src/app.css"))};\n@source ${JSON.stringify(path.join(root, "plugins/hyperfide-source-control"))};\n`,
    );
    const styled = await buildStyles({
      configFile: false,
      root: path.join(root, "apps/web"),
      plugins: [tailwindcss()],
      resolve: { alias: { "@": path.join(root, "apps/web/src") } },
      logLevel: "silent",
      build: {
        write: false,
        minify: false,
        cssMinify: false,
        rollupOptions: { input: stylesheet },
      },
    });
    assert(
      !Array.isArray(styled) && "output" in styled,
      "Application stylesheet compilation must return its actual assets",
    );
    const stylesheetAsset = styled.output.find(
      (asset) => asset.type === "asset" && asset.fileName.endsWith(".css"),
    );
    assert(
      stylesheetAsset && stylesheetAsset.source.length > 0,
      "Browser qualification requires the compiled application stylesheet",
    );
    const css = stylesheetAsset.source;
    server = createServer(async (req, res) => {
      try {
        if (req.url === "/") {
          res.setHeader("content-type", "text/html");
          res.end(
            '<!doctype html><html><head><meta name="viewport" content="width=device-width, initial-scale=1"><link rel="stylesheet" href="/app.css"></head><body><div id="root"></div><script type="module" src="/app.js"></script></body></html>',
          );
          return;
        }
        if (req.url === "/app.js") {
          res.setHeader("content-type", "text/javascript");
          res.end(await fs.readFile(bundle));
          return;
        }
        if (req.url === "/app.css") {
          res.setHeader("content-type", "text/css");
          res.end(css);
          return;
        }
        let size = 0;
        const parts = [];
        for await (const chunk of req) {
          size += chunk.length;
          if (size > 262144) throw Error("Request too large");
          parts.push(chunk);
        }
        const input = JSON.parse(Buffer.concat(parts).toString() || "{}");
        let result;
        if (req.url === "/rpc/projects")
          result = [
            {
              id: f.scope.repository_id,
              name: f.projectName ?? "Browser source project",
              configured: true,
              revisions: [{ id: base.revision_id, label: "Baseline" }],
              assignments: [],
            },
          ];
        else if (req.url === "/rpc/hostCapabilities")
          result = {
            configured: true,
            hostName: "Source fixture host",
            capabilities: await sourceHost.handlers.nativeCapabilities(
              {},
              { experimental_paths: { dataDir: temp } },
            ),
          };
        else if (req.url === "/rpc/metrics")
          result = {
            source: f.service
              ? await f.service.metrics.read(f.context, { max_rows: 1 })
              : await f.client.call("source.metrics.read", { max_rows: 1 }),
          };
        else if (req.url === "/rpc/execute") {
          const request = requestSchema.parse(input);
          assert.equal(request.projectId, f.scope.repository_id);
          result = await executeSourceAction(
            binding,
            request.operationId,
            request.action,
          );
          if (request.action.kind === "diff" && result.ok)
            diffRequests.push({
              action: request.action,
              result: result.result,
            });
          if (
            request.action.kind === "checkpoint" &&
            result.ok &&
            loseCheckpoint
          ) {
            loseCheckpoint = false;
            res.statusCode = 502;
            res.end("Mutation acknowledgment unavailable");
            return;
          }
        } else {
          res.statusCode = 404;
          res.end();
          return;
        }
        res.setHeader("content-type", "application/json");
        res.end(JSON.stringify(result));
      } catch (error) {
        console.error(
          JSON.stringify({
            stage: "source-browser-rpc",
            code: error.code ?? null,
            message: error.message,
          }),
        );
        res.statusCode = 400;
        res.end(JSON.stringify({ message: error.message }));
      }
    });
    await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
    const url = "http://127.0.0.1:" + server.address().port;
    chrome = spawn(
      "/opt/chromium/chrome-headless-shell-linux64/chrome-headless-shell",
      [
        "--no-sandbox",
        "--disable-dev-shm-usage",
        "--disable-gpu",
        "--remote-debugging-port=0",
        "--user-data-dir=" + path.join(temp, "profile"),
        "about:blank",
      ],
      { stdio: ["ignore", "ignore", "pipe"] },
    );
    let chromeLog = "";
    chrome.stderr.on("data", (chunk) => {
      chromeLog += chunk;
    });
    let port;
    for (let n = 0; n < 100; n++) {
      try {
        port = (
          await fs.readFile(
            path.join(temp, "profile/DevToolsActivePort"),
            "utf8",
          )
        ).split("\n")[0];
        break;
      } catch {
        if (chrome.exitCode !== null)
          throw Error("Chromium failed: " + chromeLog);
        await sleep(100);
      }
    }
    assert(port, "Chromium debugging endpoint did not start");
    const target = await (
      await fetch(
        "http://127.0.0.1:" + port + "/json/new?" + encodeURIComponent(url),
        { method: "PUT" },
      )
    ).json();
    socket = new WebSocket(target.webSocketDebuggerUrl);
    await new Promise((resolve, reject) => {
      socket.addEventListener("open", resolve, { once: true });
      socket.addEventListener("error", reject, { once: true });
    });
    let sequence = 0;
    const pending = new Map();
    socket.addEventListener("message", (event) => {
      const message = JSON.parse(event.data);
      if (message.id) {
        const handler = pending.get(message.id);
        pending.delete(message.id);
        if (message.error) handler?.reject(Error(message.error.message));
        else handler?.resolve(message.result);
      }
    });
    const command = (method, params = {}) =>
      new Promise((resolve, reject) => {
        const id = ++sequence;
        pending.set(id, { resolve, reject });
        socket.send(JSON.stringify({ id, method, params }));
      });
    cdp = command;
    const evaluate = async (expression) => {
      const value = await command("Runtime.evaluate", {
        expression,
        returnByValue: true,
        awaitPromise: true,
      });
      if (value.exceptionDetails)
        throw Error(
          value.exceptionDetails.text +
            JSON.stringify(value.exceptionDetails.exception),
        );
      return value.result.value;
    };
    const until = async (expression, label) => {
      for (let n = 0; n < 150; n++) {
        if (await evaluate(`document.body && (${expression})`)) return;
        await sleep(100);
      }
      throw Error(
        "Timed out: " +
          label +
          "\n" +
          (await evaluate("document.body.innerText")),
      );
    };
    const click = async (text) => {
      await until(
        `!![...document.querySelectorAll('button')].find(button=>button.textContent===${JSON.stringify(text)}&&!button.disabled)`,
        "button " + text,
      );
      await evaluate(
        `[...document.querySelectorAll('button')].find(button=>button.textContent===${JSON.stringify(text)}&&!button.disabled).click()`,
      );
    };
    const fill = async (label, value) =>
      evaluate(
        `(()=>{const label=[...document.querySelectorAll('label')].find(label=>label.firstChild?.textContent?.trim()===${JSON.stringify(label)});if(!label)throw Error('Missing field');const element=label.querySelector('input,textarea,select');const prototype=element.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:element.tagName==='SELECT'?HTMLSelectElement.prototype:HTMLInputElement.prototype;Object.getOwnPropertyDescriptor(prototype,'value').set.call(element,${JSON.stringify(value)});element.dispatchEvent(new Event('input',{bubbles:true}));element.dispatchEvent(new Event('change',{bubbles:true}));})()`,
      );
    const captureWorkspaceState = async (name) => {
      for (const viewport of widths) {
        await command("Emulation.setDeviceMetricsOverride", {
          width: viewport.width,
          height: viewport.height,
          deviceScaleFactor: 1,
          mobile: viewport.name === "mobile",
        });
        await sleep(150);
        assert(
          await evaluate(
            "document.documentElement.scrollWidth<=window.innerWidth",
          ),
          name + " horizontal overflow " + viewport.name,
        );
        const shot = await command("Page.captureScreenshot", {
          format: "png",
          captureBeyondViewport: true,
        });
        const filename = path.join(output, viewport.name + "-" + name + ".png");
        await fs.writeFile(filename, Buffer.from(shot.data, "base64"));
        screenshots.push(filename);
      }
    };
    await command("Page.enable");
    await command("Runtime.enable");
    await command("Emulation.setDeviceMetricsOverride", {
      width: 1440,
      height: 1000,
      deviceScaleFactor: 1,
      mobile: false,
    });
    await until(
      `document.body.innerText.includes(${JSON.stringify(f.projectName ?? "Browser source project")})`,
      "project list",
    );
    await until(
      `document.body.innerText.includes("Virtual filesystem:")`,
      "host capability response",
    );
    if (inspection) {
      const screenshots = [...(inspection.resume?.screenshots ?? [])];
      const capture = async (name) => {
        for (const viewport of [
          { name: "desktop", width: 1440, height: 1000 },
          { name: "tablet", width: 768, height: 1024 },
          { name: "mobile", width: 390, height: 844 },
        ]) {
          await command("Emulation.setDeviceMetricsOverride", {
            width: viewport.width,
            height: viewport.height,
            deviceScaleFactor: 1,
            mobile: viewport.name === "mobile",
          });
          await sleep(150);
          assert(
            await evaluate(
              "document.documentElement.scrollWidth<=window.innerWidth",
            ),
            name + " overflow",
          );
          const shot = await command("Page.captureScreenshot", {
            format: "png",
            captureBeyondViewport: true,
          });
          const filename = path.join(
            output,
            viewport.name + "-" + name + ".png",
          );
          await fs.writeFile(filename, Buffer.from(shot.data, "base64"));
          screenshots.push(filename);
        }
      };
      let qualification = inspection.resume?.qualification ?? null;
      if (inspection.resume?.acceptance_id)
        inspection = {
          ...inspection,
          acceptanceId: inspection.resume.acceptance_id,
        };
      const persist = async (extra = {}) => {
        if (!inspection.progressFile) return;
        await fs.writeFile(
          inspection.progressFile,
          JSON.stringify(
            {
              candidate_id: inspection.candidateId,
              revision_id: inspection.revisionId,
              root_tree_id: inspection.rootTreeId,
              acceptance_id: inspection.acceptanceId,
              qualification,
              screenshots: [...new Set(screenshots)],
              ...extra,
            },
            null,
            2,
          ) + "\n",
          { mode: 0o600 },
        );
      };
      if (inspection.candidateId && !inspection.acceptanceId) {
        assert(
          Array.isArray(inspection.evidenceIds) &&
            inspection.evidenceIds.length > 0,
        );
        await click("Changes");
        await fill("Candidate", inspection.candidateId);
        await click("Inspect candidate");
        await until(
          `document.body.innerText.includes(${JSON.stringify(inspection.candidateId)})`,
          "live candidate inspection",
        );
        await fill(
          "Independent evidence IDs (one per line)",
          inspection.evidenceIds.join("\n"),
        );
        await click("Qualify candidate");
        await until(
          'document.body.innerText.includes("qualification_id")',
          "independent candidate qualification",
        );
        qualification = await evaluate(
          "JSON.parse([...document.querySelectorAll('pre')].at(-1).textContent).result",
        );
        assert(qualification.qualification_id);
        await capture("candidate-qualified");
        await click("Accept qualified candidate");
        await until(
          'document.body.innerText.includes("acceptance_id")',
          "qualified candidate acceptance",
        );
        const result = await evaluate(
          "JSON.parse([...document.querySelectorAll('pre')].at(-1).textContent).result",
        );
        assert(result.acceptance_id);
        inspection = { ...inspection, acceptanceId: result.acceptance_id };
        await capture("candidate-accepted");
        await persist();
      }
      const accepted = await f.client.call("source.acceptance.read", {
        acceptance_id: inspection.acceptanceId,
      });
      assert.equal(accepted.revision_id, inspection.revisionId);
      assert.equal(accepted.root_tree_id, inspection.rootTreeId);
      await click("Changes");
      await fill("Revision", inspection.revisionId);
      await click("Read history");
      await until(
        `document.body.innerText.includes(${JSON.stringify(inspection.revisionId)})`,
        "exact fleet-accepted revision history",
      );
      await capture("accepted-history");
      const left = await f.createRevision({
        text: "left conflict from accepted fleet source\n",
        parent_revision_ids: [inspection.revisionId],
      });
      const right = await f.createRevision({
        text: "right conflict from accepted fleet source\n",
        parent_revision_ids: [inspection.revisionId],
      });
      const resolved = await f.createRevision({
        text: "candidate: explicit fleet conflict resolution\n",
        parent_revision_ids: [left.revision_id, right.revision_id],
      });
      assert.deepEqual(
        (await f.client.revision(left.revision_id)).parent_revision_ids,
        [inspection.revisionId],
      );
      assert.deepEqual(
        (await f.client.revision(right.revision_id)).parent_revision_ids,
        [inspection.revisionId],
      );
      assert.deepEqual(
        (await f.client.revision(resolved.revision_id)).parent_revision_ids,
        [left.revision_id, right.revision_id],
      );
      await fill("Revision", right.revision_id);
      await fill("Compare from", left.revision_id);
      await click("Merge revisions");
      await until(
        'document.body.innerText.includes("CONFLICT")',
        "conflict from exact accepted source family",
      );
      await capture("accepted-conflict");
      await fill("Resolved workspace revision", resolved.revision_id);
      await click("Apply resolved revision");
      await until(
        'document.body.innerText.includes("COMPLETED")',
        "explicit accepted-family conflict resolution",
      );
      const conflict = {
        status: "PASS",
        base_revision_id: inspection.revisionId,
        left_revision_id: left.revision_id,
        right_revision_id: right.revision_id,
        resolution_revision_id: resolved.revision_id,
        result: await evaluate(
          "JSON.parse([...document.querySelectorAll('pre')].at(-1).textContent).result",
        ),
      };
      assert.equal(conflict.result.status, "COMPLETED");
      await capture("accepted-resolution");
      await click("Publication");
      await fill("Acceptance receipt", inspection.acceptanceId);
      await fill("Configured destination", inspection.destinationId);
      await fill("Target reference", inspection.targetRef ?? "refs/heads/main");
      if (inspection.expectedOldOid)
        await fill(
          "Expected remote Git identity (empty for new reference)",
          inspection.expectedOldOid,
        );
      if (inspection.resume?.publication_operation_id) {
        await fill(
          "Publication operation",
          inspection.resume.publication_operation_id,
        );
      } else {
        await click("Create publication");
        await until(
          'document.body.innerText.includes("QUEUED")',
          "fleet accepted publication intent",
        );
        const queued = await evaluate(
          "JSON.parse([...document.querySelectorAll('pre')].at(-1).textContent).result",
        );
        await persist({ publication_operation_id: queued.operation_id });
      }
      await click("Reconcile publication");
      await until(
        'document.body.innerText.includes("PUBLISHED")',
        "fleet accepted publication readback",
      );
      const published = await evaluate(
        "JSON.parse([...document.querySelectorAll('pre')].at(-1).textContent).result",
      );
      assert.equal(published.revision_id, inspection.revisionId);
      assert.match(published.git_oid, /^git:sha1:[a-f0-9]{40}$/);
      await capture("accepted-publication");
      const receipt = {
        status: "PASS",
        scope:
          "actual SourcePanel in Chromium, supplied fleet acceptance, real native API and Git publication",
        acceptance_id: inspection.acceptanceId,
        revision_id: inspection.revisionId,
        root_tree_id: inspection.rootTreeId,
        publication: published,
        qualification,
        conflict,
        screenshots: [...new Set(screenshots)],
        checks: [
          "display exact supplied accepted revision history",
          "resolve an actual merge conflict derived from the supplied accepted revision",
          "publish exact supplied fleet acceptance through product UI",
          "read final publication revision and Git identity",
          "desktop tablet mobile have no horizontal overflow",
        ],
      };
      await fs.writeFile(
        path.join(output, "receipt.json"),
        JSON.stringify(receipt, null, 2) + "\n",
      );
      return receipt;
    }
    await fill("Paths (one per line)", "main.txt");
    await click("Check dependency closure");
    await until(
      `document.querySelector('[data-source-closure="READY"]') !== null`,
      "dependency closure verified from native tree",
    );
    await click("Open workspace");
    await until(
      'document.body.innerText.includes("Generation: 1")',
      "workspace admitted",
    );
    await fill("File path", "main.txt");
    await click("Load file from revision");
    await until(
      `document.querySelector("textarea.font-mono")?.value === ${JSON.stringify(baselineContent)}`,
      "pinned file content",
    );
    await fill("File content", "candidate: browser edit\n");
    await until(
      `document.querySelector('[data-source-workspace-state="DIRTY"]') !== null`,
      "local edit remains dirty",
    );
    assert.equal(
      await evaluate(
        'document.body.innerText.includes("Receipt-backed revision:")',
      ),
      false,
    );
    await click("Save durable checkpoint");
    await until(
      'document.body.innerText.includes("Unresolved checkpoint operation")',
      "lost checkpoint response",
    );
    const pendingId = await evaluate(
      `JSON.parse(sessionStorage.getItem('hyperfide.source.pending.v1')).operationId`,
    );
    assert.match(pendingId, /^[a-f0-9-]{36}$/);
    const committedBeforeReceipt = await binding.client.operation(pendingId);
    assert.equal(committedBeforeReceipt.state, "SUCCEEDED");
    await command("Page.reload", { ignoreCache: true });
    await until(
      `document.body.innerText.includes("Unresolved checkpoint operation") && document.querySelector('[data-source-workspace-state="UPLOADING"]') !== null`,
      "reconnect retains ambiguous checkpoint as uploading",
    );
    await until(
      'document.body.innerText.includes("Generation: 2")',
      "reconnect can observe the workspace without acknowledging the retained edit",
    );
    assert.equal(
      await evaluate(
        'document.querySelector("[data-source-workspace-state]").dataset.sourceWorkspaceState',
      ),
      "UPLOADING",
    );
    assert.equal(
      await evaluate(
        'document.body.innerText.includes("Receipt-backed revision:")',
      ),
      false,
    );
    assert.equal(
      await evaluate(
        'document.body.innerText.includes("Checkpoint committed")',
      ),
      false,
    );
    assert.equal(
      await evaluate('document.querySelector("textarea.font-mono").value'),
      "candidate: browser edit\n",
    );
    assert.equal(
      await evaluate(
        `JSON.parse(sessionStorage.getItem('hyperfide.source.pending.v1')).operationId`,
      ),
      pendingId,
    );
    await captureWorkspaceState("checkpoint-unresolved");
    await click("Retry same operation");
    await until(
      `sessionStorage.getItem('hyperfide.source.pending.v1')===null`,
      "checkpoint replay",
    );
    await until(
      `document.querySelector('[data-source-workspace-state="ACKNOWLEDGED"]') !== null`,
      "checkpoint becomes acknowledged only after its bound receipt arrives",
    );
    assert(
      await evaluate(
        'document.body.innerText.includes("Receipt-backed revision:")',
      ),
    );
    await captureWorkspaceState("checkpoint-acknowledged");
    await click("Refresh workspace");
    await until(
      'document.body.innerText.includes("Generation: 2")',
      "durable checkpoint generation",
    );
    const durable = await binding.client.operation(pendingId);
    assert.equal(durable.state, "SUCCEEDED");
    const changedRevision = durable.result.revision_id;
    await click("Changes");
    await fill("Revision", changedRevision);
    await click("Read history");
    await until(
      'document.body.innerText.includes("Workspace checkpoint")',
      "history",
    );
    await fill("Compare from", base.revision_id);
    await click("Compare revisions");
    await until(
      'document.body.innerText.includes("MODIFIED")',
      "revision diff",
    );
    assert(
      f.productPagination,
      "Product actor must provide its real oversized dataset",
    );
    await fill("Revision", f.productPagination.revisionId);
    await fill("Compare from", f.productPagination.beforeRevisionId);
    await click("Compare revisions");
    await until(
      'document.body.innerText.includes("Next changes page")',
      "oversized diff exposes next page",
    );
    const visiblePaths = [],
      seenCursors = new Set();
    let pages = 0;
    for (;;) {
      const current = diffRequests.at(-1);
      assert.equal(
        current.action.afterRevisionId,
        f.productPagination.revisionId,
      );
      const visible = await evaluate(
        `[...document.querySelectorAll('main p')].filter(element=>element.querySelector('strong')).map(element=>element.textContent.replace(/^(ADDED|REMOVED|MODIFIED|TYPE_CHANGED)\\s+/,''))`,
      );
      assert.deepEqual(
        visible,
        current.result.entries.map((entry) => entry.path),
      );
      visiblePaths.push(...visible);
      pages++;
      assert(pages <= 4, "UI diff pagination must finish within the dataset");
      if (!current.result.next_cursor) {
        assert.equal(current.result.coverage.state, "COMPLETE");
        assert.equal(
          await evaluate(
            '[...document.querySelectorAll("button")].some(button=>button.textContent==="Next changes page")',
          ),
          false,
        );
        break;
      }
      assert.equal(current.result.coverage.state, "PARTIAL");
      assert(!seenCursors.has(current.result.next_cursor));
      seenCursors.add(current.result.next_cursor);
      const priorCount = diffRequests.length;
      await click("Next changes page");
      for (
        let attempt = 0;
        diffRequests.length === priorCount && attempt < 150;
        attempt++
      )
        await sleep(100);
      assert.equal(diffRequests.length, priorCount + 1);
      assert.equal(
        diffRequests.at(-1).action.cursor,
        current.result.next_cursor,
      );
      const nextPath = diffRequests.at(-1).result.entries[0].path;
      await until(
        `!![...document.querySelectorAll('main p')].find(element=>element.querySelector('strong')&&element.textContent.endsWith(${JSON.stringify(nextPath)}))`,
        "continued diff rendered",
      );
    }
    assert(pages > 1);
    assert.deepEqual(
      [...visiblePaths].sort(),
      f.productPagination.expectedPaths,
    );
    assert.equal(
      new Set(visiblePaths).size,
      f.productPagination.expectedPaths.length,
    );
    await captureWorkspaceState("diff-complete");
    const left = await f.createRevision({ text: "left browser change\n" }),
      right = await f.createRevision({ text: "right browser change\n" }),
      resolved = await f.createRevision({ text: "resolved browser change\n" });
    await fill("Revision", right.revision_id);
    await fill("Compare from", left.revision_id);
    await click("Merge revisions");
    await until(
      'document.body.innerText.includes("CONFLICT")',
      "merge conflict",
    );
    await fill("Resolved workspace revision", resolved.revision_id);
    await click("Apply resolved revision");
    await until(
      'document.body.innerText.includes("COMPLETED")',
      "conflict resolution",
    );
    const reviewed = {
        revision_id: changedRevision,
        root_tree_id: durable.result.workspace.root_tree_id,
        change_id: durable.result.receipt.revision_receipt.change_id,
      },
      candidate = await f.createCandidate(reviewed, {
        domain: await f.createDomain(reviewed),
      }),
      proof = await f.publishEvidence(candidate);
    await fill("Candidate", candidate.candidate_id);
    await fill(
      "Independent evidence IDs (one per line)",
      proof.evidence.evidence_id,
    );
    await click("Qualify candidate");
    await until(
      '[...document.querySelectorAll("label")].find(label=>label.textContent.trim()==="Qualification")?.querySelector("input")?.value.startsWith("sha256:")',
      "qualification response",
    );
    await click("Accept qualified candidate");
    await until(
      'document.body.innerText.includes("Revision accepted")',
      "acceptance receipt",
    );
    const accepted = await evaluate(
      `JSON.parse([...document.querySelectorAll('pre')].at(-1).textContent).result.acceptance_id`,
    );
    assert.match(accepted, /^sha256:/);
    await click("Workspace");
    await fill("Workspace", durable.result.workspace.workspace_id);
    await click("Refresh workspace");
    await until(
      `document.querySelector('[data-source-workspace-state="ACCEPTED"]') !== null`,
      "workspace accepted state is bound to the server acceptance receipt",
    );
    await captureWorkspaceState("workspace-accepted");
    await click("Changes");
    for (const viewport of widths) {
      await command("Emulation.setDeviceMetricsOverride", {
        width: viewport.width,
        height: viewport.height,
        deviceScaleFactor: 1,
        mobile: viewport.name === "mobile",
      });
      await sleep(150);
      assert(
        await evaluate(
          "document.documentElement.scrollWidth<=window.innerWidth",
        ),
        "Horizontal overflow " + viewport.name,
      );
      const screenshot = await command("Page.captureScreenshot", {
        format: "png",
        captureBeyondViewport: true,
      });
      const filename = path.join(output, viewport.name + ".png");
      await fs.writeFile(filename, Buffer.from(screenshot.data, "base64"));
      screenshots.push(filename);
    }
    await click("Publication");
    await fill("Acceptance receipt", accepted);
    await fill("Configured destination", "product-browser");
    await fill("Target reference", "refs/heads/main");
    await click("Create publication");
    await until(
      'document.body.innerText.includes("QUEUED")',
      "publication intent",
    );
    await click("Reconcile publication");
    await until(
      'document.body.innerText.includes("PUBLISHED")',
      "remote publication",
    );
    const publishedIdentity = await evaluate(
      `JSON.parse([...document.querySelectorAll('pre')].at(-1).textContent).result.git_oid`,
    );
    assert.match(publishedIdentity, /^git:sha1:[a-f0-9]{40}$/);
    await fill(
      "Expected remote Git identity (empty for new reference)",
      publishedIdentity,
    );
    await click("Create publication");
    await until(
      'document.body.innerText.includes("QUEUED")',
      "guarded publication intent",
    );
    await click("Reconcile publication");
    await until(
      'document.body.innerText.includes("PUBLISHED")',
      "guarded publication readback",
    );
    for (const viewport of widths) {
      await command("Emulation.setDeviceMetricsOverride", {
        width: viewport.width,
        height: viewport.height,
        deviceScaleFactor: 1,
        mobile: viewport.name === "mobile",
      });
      await sleep(150);
      assert(
        await evaluate(
          "document.documentElement.scrollWidth<=window.innerWidth",
        ),
        "Publication horizontal overflow " + viewport.name,
      );
      const screenshot = await command("Page.captureScreenshot", {
        format: "png",
        captureBeyondViewport: true,
      });
      const filename = path.join(output, viewport.name + "-publication.png");
      await fs.writeFile(filename, Buffer.from(screenshot.data, "base64"));
      screenshots.push(filename);
    }
    await click("Workspace");
    await fill("Workspace", durable.result.workspace.workspace_id);
    await click("Refresh workspace");
    await until(
      `document.querySelector('[data-source-workspace-state="EXPORTED"]') !== null`,
      "workspace exported state requires remote publication readback",
    );
    await captureWorkspaceState("workspace-exported");
    const blob = await f.blob(Buffer.alloc(1100000, 42));
    const transferId = crypto.randomUUID(),
      created = await binding.client.call("source.transfers.create", {
        transfer_id: transferId,
        direction: "DOWNLOAD",
        object_id: blob,
        expires_at: new Date(Date.now() + 120000).toISOString(),
      });
    const grantId = created.grant.url.split("/").at(-1);
    await f.service.transfers.redeem({
      grant_id: grantId,
      token: created.grant.token,
      url: created.grant.url,
      method: "GET",
      range: "bytes=0-1048575",
    });
    await click("Transfers");
    await fill("Transfer", transferId);
    await click("Refresh transfer");
    await until(
      'document.body.innerText.includes("1048576 / ")',
      "partial transfer progress",
    );
    assert.equal(
      await evaluate(
        `document.body.textContent.includes(${JSON.stringify(created.grant.token)})`,
      ),
      false,
    );
    await f.service.transfers.redeem({
      grant_id: grantId,
      token: created.grant.token,
      url: created.grant.url,
      method: "GET",
      range: `bytes=1048576-${Number(created.size_bytes) - 1}`,
    });
    await click("Verify completion");
    await until(
      'document.body.innerText.includes("Download complete")',
      "completed download",
    );
    for (const viewport of widths) {
      await command("Emulation.setDeviceMetricsOverride", {
        width: viewport.width,
        height: viewport.height,
        deviceScaleFactor: 1,
        mobile: viewport.name === "mobile",
      });
      await sleep(150);
      assert(
        await evaluate(
          "document.documentElement.scrollWidth<=window.innerWidth",
        ),
        "Transfer overflow " + viewport.name,
      );
      const shot = await command("Page.captureScreenshot", {
        format: "png",
        captureBeyondViewport: true,
      });
      const filename = path.join(output, viewport.name + "-transfer.png");
      await fs.writeFile(filename, Buffer.from(shot.data, "base64"));
      screenshots.push(filename);
    }
    await click("Metrics");
    await until(
      'document.body.innerText.includes("observed records")',
      "bounded source metrics",
    );
    assert(
      await evaluate('document.body.innerText.includes("partial coverage")'),
    );
    assert(
      await evaluate(
        'document.body.innerText.includes("Each family samples up to 50 records")',
      ),
    );
    assert(
      await evaluate('document.body.innerText.includes("Product requests")'),
    );
    assert(
      await evaluate("document.documentElement.scrollWidth<=window.innerWidth"),
    );
    const metricsShot = await command("Page.captureScreenshot", {
      format: "png",
      captureBeyondViewport: true,
    });
    const metricsFilename = path.join(output, "mobile-metrics.png");
    await fs.writeFile(
      metricsFilename,
      Buffer.from(metricsShot.data, "base64"),
    );
    screenshots.push(metricsFilename);
    const receipt = {
      status: "PASS",
      scope:
        "actual SourcePanel in Chromium with an HTTP bridge to the real product action adapter and native source service; project metadata is a declared fixture",
      screenshots,
      checks: [
        "open scoped workspace through the browser",
        "edit and checkpoint with deliberate lost acknowledgment",
        "reconnect retains uploading state even when the server has committed, until the original checkpoint receipt arrives",
        "reload the same operation without duplicate checkpoint",
        "page an oversized diff through next_cursor to complete UI coverage without gaps or duplicates",
        "inspect real revision history and changed paths",
        "resolve a real merge conflict and accept independently qualified source",
        "publish accepted source through actual local Git transport",
        "inspect partial transfer progress and verify completion without exposing its grant",
        "actual host capability result is visible without implicit fallback",
        "metrics show actual bounded source samples and partial coverage",
        "desktop tablet and mobile widths have no horizontal overflow",
      ],
    };
    await fs.writeFile(
      path.join(output, "receipt.json"),
      JSON.stringify(receipt, null, 2) + "\n",
    );
    return receipt;
  } catch (error) {
    if (cdp)
      try {
        const shot = await cdp("Page.captureScreenshot", {
          format: "png",
          captureBeyondViewport: true,
        });
        await fs.writeFile(
          path.join(output, "failure.png"),
          Buffer.from(shot.data, "base64"),
        );
      } catch {}
    throw error;
  } finally {
    socket?.close();
    if (chrome) {
      chrome.kill("SIGTERM");
      await new Promise((resolve) => {
        if (chrome.exitCode !== null) resolve();
        else chrome.once("exit", resolve);
      });
    }
    if (server) await new Promise((resolve) => server.close(resolve));
    await fs.rm(temp, { recursive: true, force: true });
  }
}

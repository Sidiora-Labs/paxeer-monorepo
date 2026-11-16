import assert from "node:assert/strict";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, extname, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import test from "node:test";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";
import {
  classifyError,
  makeError,
} from "@paxeer-network/paxeer-backend/core/errors/classify";
import { useToast } from "@paxeer-network/paxeer-backend/ui/atoms/use-toast";
import { Toaster } from "@paxeer-network/paxeer-backend/ui/atoms/toaster";
import * as widgets from "@paxeer-network/paxeer-backend/widgets/errors";
import { reportModalStore } from "@paxeer-network/paxeer-backend/widgets/errors/ReportFeedbackModal";

const packageRoot = fileURLToPath(
  new URL("../packages/paxeer-backend/", import.meta.url),
);
const widgetRoot = resolve(packageRoot, "src/widgets/errors");
const manifest = JSON.parse(
  readFileSync(resolve(packageRoot, "package.json"), "utf8"),
) as { name: string; exports: Record<string, string> };

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = resolve(directory, entry.name);
    return entry.isDirectory()
      ? sourceFiles(path)
      : /\.[cm]?[jt]sx?$/.test(entry.name)
        ? [path]
        : [];
  });
}

function resolvesToFile(path: string): boolean {
  const candidates = [
    path,
    ...[".ts", ".tsx", ".js", ".jsx", ".json"].map(
      (extension) => path + extension,
    ),
    ...["index.ts", "index.tsx", "index.js"].map((index) =>
      resolve(path, index),
    ),
  ];
  return candidates.some((candidate) => {
    try {
      return statSync(candidate).isFile();
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") return false;
      throw error;
    }
  });
}

async function errorWidgetImportsResolve() {
  const files = sourceFiles(widgetRoot);
  assert.ok(files.length >= 5);
  let checked = 0;
  for (const path of files) {
    const source = ts.createSourceFile(
      path,
      readFileSync(path, "utf8"),
      ts.ScriptTarget.Latest,
      true,
      extname(path) === ".tsx" ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
    );
    function checkSpecifier(specifier: string) {
      if (specifier.startsWith(".")) {
        assert.ok(
          resolvesToFile(resolve(dirname(path), specifier)),
          `${path}: ${specifier}`,
        );
        checked++;
      } else if (specifier.startsWith(`${manifest.name}/`)) {
        const subpath = `.${specifier.slice(manifest.name.length)}`;
        assert.equal(
          typeof manifest.exports[subpath],
          "string",
          `${path}: ${subpath}`,
        );
        assert.ok(
          resolvesToFile(resolve(packageRoot, manifest.exports[subpath])),
          `${path}: ${subpath}`,
        );
        checked++;
      }
    }
    function visit(node: ts.Node) {
      if (
        (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
        node.moduleSpecifier &&
        ts.isStringLiteral(node.moduleSpecifier)
      ) {
        checkSpecifier(node.moduleSpecifier.text);
      } else if (
        ts.isCallExpression(node) &&
        (node.expression.kind === ts.SyntaxKind.ImportKeyword ||
          (ts.isIdentifier(node.expression) &&
            node.expression.text === "require")) &&
        node.arguments[0] &&
        ts.isStringLiteral(node.arguments[0])
      ) {
        checkSpecifier(node.arguments[0].text);
      } else if (
        ts.isImportTypeNode(node) &&
        ts.isLiteralTypeNode(node.argument) &&
        ts.isStringLiteral(node.argument.literal)
      ) {
        checkSpecifier(node.argument.literal.text);
      }
      ts.forEachChild(node, visit);
    }
    visit(source);
  }
  assert.ok(checked > files.length);
  for (const [subpath, target] of Object.entries(manifest.exports)) {
    if (subpath.startsWith("./widgets/errors/")) {
      assert.ok(resolvesToFile(resolve(packageRoot, target)), subpath);
      assert.ok(await import(pathToFileURL(resolve(packageRoot, target)).href));
    }
  }
}

function errorWidgetIndexLoads() {
  for (const name of [
    "showErrorToast",
    "toastError",
    "ErrorPanel",
    "ReportFeedbackModalHost",
    "openReportModal",
    "closeReportModal",
  ] as const) {
    assert.equal(typeof widgets[name], "function", name);
  }
  assert.ok(widgets.ToastAction);
  assert.equal(
    renderToStaticMarkup(React.createElement(widgets.ReportFeedbackModalHost)),
    "",
  );
}

function readToastQueue() {
  let state: ReturnType<typeof useToast> | undefined;
  function ToastQueueReader() {
    state = useToast();
    return null;
  }
  renderToStaticMarkup(React.createElement(ToastQueueReader));
  assert.ok(state);
  return state.toasts;
}

function elementText(node: React.ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(elementText).join("");
  if (React.isValidElement<{ children?: React.ReactNode }>(node))
    return elementText(node.props.children);
  return "";
}

type ActionProps = {
  altText?: string;
  onClick?: () => void;
  children?: React.ReactNode;
};

function toastActions(
  node: React.ReactNode,
): React.ReactElement<ActionProps>[] {
  if (Array.isArray(node)) return node.flatMap(toastActions);
  if (!React.isValidElement<ActionProps>(node)) return [];
  return node.type === widgets.ToastAction
    ? [node]
    : toastActions(node.props.children);
}

function classifiedErrorsReachToastActions() {
  let retries = 0;
  const onRetry = () => {
    retries++;
  };
  const retryable = classifyError(new TypeError("Failed to fetch"));
  const notRetryable = makeError("API_FORBIDDEN");
  assert.equal(retryable.isRetryable, true);
  assert.equal(notRetryable.isRetryable, false);

  for (const error of [retryable, notRetryable, makeError("UNKNOWN")]) {
    for (const handler of [undefined, onRetry]) {
      const before = readToastQueue();
      const handle = widgets.showErrorToast(error, { onRetry: handler });
      const queue = readToastQueue();
      const added = queue.filter(
        (toast) => !before.some((previous) => previous.id === toast.id),
      );
      assert.equal(added.length, 1);
      const [toast] = added;
      assert.equal(toast.id, handle.id);
      assert.equal(toast.open, true);
      assert.equal(elementText(toast.title), error.title);
      assert.ok(elementText(toast.description).includes(error.message));
      if (error.suggestedFix)
        assert.ok(elementText(toast.description).includes(error.suggestedFix));
      assert.equal(
        toast.variant,
        error.severity === "error" || error.severity === "fatal"
          ? "destructive"
          : "default",
      );
      const actions = toastActions(toast.action);
      const retry = actions.find(
        (action) => action.props.altText === "Retry the failed action",
      );
      const report = actions.find(
        (action) => action.props.altText === "Report this issue",
      );
      assert.equal(Boolean(retry), error.isRetryable && Boolean(handler));
      assert.equal(Boolean(report), error.isReportable);
      assert.equal(
        actions.length,
        Number(Boolean(retry)) + Number(error.isReportable),
      );
      if (retry) {
        const count = retries;
        assert.equal(typeof retry.props.onClick, "function");
        retry.props.onClick!();
        assert.equal(retries, count + 1);
        const html = renderToStaticMarkup(retry);
        assert.match(html, /<button/);
        assert.match(html, /type="button"/);
        assert.match(html, /aria-label="Retry the failed action"/);
        assert.doesNotMatch(html, /\sdisabled(?:\s|=|\/?>)|\stabindex="-1"/);
      }
      if (report) {
        report.props.onClick!();
        assert.equal(reportModalStore.getSnapshot()?.error, error);
        widgets.closeReportModal();
      }
    }
  }
  const html = renderToStaticMarkup(React.createElement(Toaster));
  assert.match(html, /aria-label="Notifications"/);
  assert.match(html, /aria-label="Retry the failed action"/);
  assert.match(html, /aria-label="Report this issue"/);
}

function reportModalPublishesOpenAndCloseState() {
  widgets.closeReportModal();
  const snapshots: ReturnType<typeof reportModalStore.getSnapshot>[] = [];
  const unsubscribe = reportModalStore.subscribe(() =>
    snapshots.push(reportModalStore.getSnapshot()),
  );
  try {
    const error = classifyError(new Error("Unexpected response"));
    const other = makeError("API_SERVER_ERROR");
    widgets.openReportModal(error);
    const first = reportModalStore.getSnapshot();
    assert.equal(first?.error, error);
    assert.equal(first?.returnFocus, null);
    assert.equal(reportModalStore.getSnapshot(), first);
    widgets.openReportModal(other);
    assert.equal(reportModalStore.getSnapshot()?.error, other);
    widgets.closeReportModal();
    assert.equal(reportModalStore.getSnapshot(), null);
    assert.equal(snapshots.length, 3);
    assert.deepEqual(
      snapshots.map((snapshot) => snapshot?.error ?? null),
      [error, other, null],
    );
  } finally {
    unsubscribe();
    widgets.closeReportModal();
  }
  assert.equal(snapshots.length, 3);
}

function errorPanelRendersClassifiedContentAndControls() {
  const retryable = makeError("UNKNOWN");
  const panel = renderToStaticMarkup(
    React.createElement(widgets.ErrorPanel, { error: retryable, onRetry() {} }),
  );
  assert.match(panel, /role="alert"/);
  assert.ok(panel.includes(retryable.title));
  assert.ok(
    panel.includes(
      renderToStaticMarkup(
        React.createElement(React.Fragment, null, retryable.message),
      ),
    ),
  );
  assert.ok(panel.includes(retryable.suggestedFix!));
  assert.match(panel, />Retry<\/button>/);
  assert.match(panel, />Report<\/button>/);
  const withoutHandler = renderToStaticMarkup(
    React.createElement(widgets.ErrorPanel, { error: retryable }),
  );
  assert.doesNotMatch(withoutHandler, />Retry<\/button>/);
  const forbidden = makeError("API_FORBIDDEN");
  assert.equal(forbidden.isRetryable, false);
  const withoutRetry = renderToStaticMarkup(
    React.createElement(widgets.ErrorPanel, { error: forbidden, onRetry() {} }),
  );
  assert.doesNotMatch(withoutRetry, />Retry<\/button>/);
  const benign = makeError("RUNTIME_ABORTED");
  assert.equal(benign.isReportable, false);
  assert.equal(benign.suggestedFix, null);
  const withoutReport = renderToStaticMarkup(
    React.createElement(widgets.ErrorPanel, { error: benign }),
  );
  assert.doesNotMatch(withoutReport, />Report<\/button>|Try this:/);
}

test(errorWidgetImportsResolve);
test(errorWidgetIndexLoads);
test(classifiedErrorsReachToastActions);
test(reportModalPublishesOpenAndCloseState);
test(errorPanelRendersClassifiedContentAndControls);

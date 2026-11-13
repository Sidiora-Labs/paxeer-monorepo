// Task 22.33.506 closure record:
// bottom-tabs-navigation-length -> bottom_tabs_preserve_every_destination
// button-slot-loading-children -> button_slot_loading_preserves_content_and_interactions
// wizard-empty-steps -> wizard_empty_and_replaced_steps_reconcile_real_state
// Wiring requests: none; all three repairs belong to the component domain.
// Baseline verification was not run: the bounded gate order requires complete
// implementation before hf-build/hf-done and forbids a manual verify run.
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith("@layerx/ui/")) {
      const source = new URL(
        `../packages/layerx-ui/src/${specifier.slice("@layerx/ui/".length)}`,
        import.meta.url,
      );
      for (const suffix of [".ts", ".tsx"]) {
        const candidate = new URL(source.href + suffix);
        if (existsSync(candidate)) return nextResolve(candidate.href, context);
      }
    }
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: {
            target: ts.ScriptTarget.ES2022,
            module: ts.ModuleKind.ESNext,
            jsx: ts.JsxEmit.ReactJSX,
          },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { AppShell } = await import("@layerx/ui/components/app-shell");
const { Button } = await import("@layerx/ui/components/button");
const { Wizard } = await import("@layerx/ui/components/wizard");
const { PlatformProvider } = await import("@layerx/ui/lib/platform");

type ElementProps = Record<string, unknown> & { children?: React.ReactNode };

function elements(node: React.ReactNode): React.ReactElement<ElementProps>[] {
  return React.Children.toArray(node).flatMap((child) => {
    if (!React.isValidElement<ElementProps>(child)) return [];
    return [child, ...elements(child.props.children)];
  });
}

function bottom_tabs_preserve_every_destination() {
  for (const count of [0, 1, 2, 3, 4, 5, 9]) {
    const items = Array.from({ length: count }, (_, index) => ({
      id: `destination-${index}`,
      label: `Destination ${index}`,
      badge: index === count - 1 ? 7 : 0,
    }));
    const html = renderToStaticMarkup(React.createElement(PlatformProvider, {
      value: "mobile",
      children: React.createElement(AppShell, {
        nav: items,
        activeNav: items.at(-1)?.id ?? "",
        children: React.createElement("p", null, "Page content"),
      }),
    }));
    const nav = html.match(/<nav aria-label="Primary"[\s\S]*?<\/nav>/u)?.[0];
    assert.ok(nav, `primary navigation for ${count} destinations`);
    assert.match(nav, /overflow-x-auto/u);
    assert.equal((nav.match(/<button\b/gu) ?? []).length, count + 1);
    assert.equal((nav.match(/aria-current="page"/gu) ?? []).length, count === 0 ? 0 : 1);
    for (const item of items) {
      assert.equal(nav.split(`>${item.label}</span>`).length - 1, 1, item.id);
    }
    if (count > 0) assert.match(nav, />7<\/span>/u);
    assert.match(nav, /aria-label="New action"/u);
    assert.match(html, /Page content/u);
  }
}

function button_slot_loading_preserves_content_and_interactions() {
  for (const loading of [false, true]) {
    for (const disabled of [false, true]) {
      let delivered: ElementProps | undefined;
      let childClicks = 0;
      let parentClicks = 0;
      let childCaptures = 0;
      let childKeys = 0;
      // This fixture consumes the props produced by the real Radix Slot and
      // renders its real anchor, including the content composed by Slot.
      function Destination(props: ElementProps) {
        delivered = props;
        return React.createElement("a", props, props.children);
      }
      const html = renderToStaticMarkup(React.createElement(Button, {
        asChild: true,
        loading,
        disabled,
        onClick: () => { parentClicks += 1; },
        children: React.createElement(Destination, {
          href: "/destination",
          className: "destination",
          tabIndex: 0,
          disabled: false,
          onClick: () => { childClicks += 1; },
          onClickCapture: () => { childCaptures += 1; },
          onKeyDown: () => { childKeys += 1; },
          onKeyDownCapture: () => { childKeys += 1; },
          children: React.createElement("span", null, "Open destination"),
        }),
      }));
      assert.ok(delivered);
      assert.equal((html.match(/<a\b/gu) ?? []).length, 1);
      assert.equal((html.match(/<svg\b/gu) ?? []).length, loading ? 1 : 0);
      assert.match(html, /<span>Open destination<\/span>/u);
      assert.match(html, /href="\/destination"/u);
      assert.match(String(delivered.className), /destination/u);
      assert.match(String(delivered.className), /inline-flex/u);
      assert.equal(delivered["aria-busy"], loading ? true : undefined);
      const blocked = loading || disabled;
      assert.equal(delivered.disabled, blocked);
      assert.equal(delivered.tabIndex, blocked ? -1 : 0);
      assert.equal(delivered["aria-disabled"], blocked ? true : undefined);
      for (const name of ["onClickCapture", "onClick", "onKeyDownCapture", "onKeyDown"]) {
        const handler: unknown = delivered[name];
        assert.equal(typeof handler, "function");
        if (typeof handler !== "function") throw new TypeError(`Missing ${name}`);
        const event = new Event(name.includes("Key") ? "keydown" : "click", {
          bubbles: true,
          cancelable: true,
        });
        handler(event);
        assert.equal(event.defaultPrevented, blocked, name);
        assert.equal(event.cancelBubble, blocked, name);
      }
      assert.equal(childClicks, blocked ? 0 : 1);
      assert.equal(parentClicks, blocked ? 0 : 1);
      assert.equal(childCaptures, blocked ? 0 : 1);
      assert.equal(childKeys, blocked ? 0 : 2);
    }
  }
  for (const loading of [false, true]) {
    const html = renderToStaticMarkup(React.createElement(Button, {
      loading,
      children: "Save",
    }));
    assert.match(html, /^<button\b/u);
    assert.equal((html.match(/<svg\b/gu) ?? []).length, loading ? 1 : 0);
    assert.equal(html.includes('disabled=""'), loading);
    assert.match(html, /Save<\/button>/u);
  }
}

function wizard_empty_and_replaced_steps_reconcile_real_state() {
  for (const platform of ["mobile", "desktop"] as const) {
    let completed = 0;
    let cancelled = 0;
    const steps = ["first", "second", "third"].map((id) => ({
      id,
      label: id,
      title: `${id} decision`,
      render: () => React.createElement("input", { name: id }),
    }));
    for (const empty of [[], steps.slice(0, 1)]) {
      const html = renderToStaticMarkup(React.createElement(Wizard, {
        platform,
        steps: empty,
        onComplete: () => { completed += 1; },
        onCancel: () => { cancelled += 1; },
      }));
      if (empty.length === 0) {
        assert.match(html, /role="status">No steps available\./u);
        assert.doesNotMatch(html, /Continue|Confirm|NaN|1\/0/u);
        assert.match(html, /Cancel/u);
      } else {
        assert.match(html, /first decision/u);
        assert.match(html, /Confirm/u);
      }
    }
    let phase = 0;
    const observed: string[] = [];
    // The fixture calls the production hooked component in React's render
    // context. Render-phase updates keep its actual useState alive across
    // navigation and prop replacements, without a substitute hook or renderer.
    function Workflow() {
      const [, refresh] = React.useState(0);
      const currentSteps = phase === 3 ? steps.slice(0, 1) : phase === 5 ? [] : steps;
      const output = Wizard({
        steps: currentSteps,
        platform,
        onComplete: () => { completed += 1; },
        onCancel: () => { cancelled += 1; },
      });
      const nodes = elements(output);
      const heading = nodes.find((node) => node.type === "h2");
      observed.push(String(heading?.props.children ?? "empty"));
      const forward = nodes.find((node) => node.props.children === "Continue" || node.props.children === "Confirm");
      const backward = nodes.find((node) => node.props["aria-label"] === "Back" || node.props.children === "Cancel" || node.props.children === "Back");
      function click(node: React.ReactElement<ElementProps> | undefined) {
        assert.ok(node);
        assert.equal(typeof node.props.onClick, "function");
        if (typeof node.props.onClick !== "function") throw new TypeError("Missing wizard action");
        node.props.onClick();
      }
      const turn = phase++;
      if ([0, 1, 7, 10, 11, 12].includes(turn)) click(forward);
      else if ([8, 9].includes(turn)) click(backward);
      if ([2, 3, 4, 5, 6, 9].includes(turn)) refresh((value) => value + 1);
      return output;
    }
    const finalHtml = renderToStaticMarkup(React.createElement(Workflow));
    assert.deepEqual(observed, [
      "first decision", "second decision", "third decision",
      "first decision", "first decision", "empty", "first decision",
      "first decision", "second decision", "first decision",
      "first decision", "second decision", "third decision",
    ]);
    assert.match(finalHtml, /third decision/u);
    assert.equal(completed, 1);
    assert.equal(cancelled, 1);

    const invalidHtml = renderToStaticMarkup(React.createElement(Wizard, {
      platform,
      steps: [{ ...steps[0]!, canContinue: () => false }],
    }));
    assert.match(invalidHtml, /<button[^>]*disabled=""[^>]*>Confirm<\/button>/u);
  }
}

test(bottom_tabs_preserve_every_destination);
test(button_slot_loading_preserves_content_and_interactions);
test(wizard_empty_and_replaced_steps_reconcile_real_state);

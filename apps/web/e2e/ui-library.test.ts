import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { runInNewContext } from "node:vm";
import { cn } from "@layerx/ui/cn";
import { buttonVariants, type ButtonProps } from "@layerx/ui/components/button";
import { SourceMapConsumer, SourceMapGenerator } from "source-map";

import { human_design_tokens } from "../src/design/tokens.ts";
import { formatCommonJs, formatCommonJsMap } from "../packages/layerx-ui/format-commonjs.ts";

const themeSource = readFileSync(
  new URL("../packages/layerx-ui/src/styles/tokens.css", import.meta.url),
  "utf8",
);

function themeUtilityNames(prefix: "text" | "shadow" | "color"): string[] {
  const names = new Set<string>();
  for (const match of themeSource.matchAll(new RegExp(`^\\s*--${prefix}-([a-z0-9]+(?:-[a-z0-9]+)*):`, "gmu"))) {
    const name = match[1];
    assert.ok(name);
    names.add(name);
  }
  assert.ok(names.size > 0, `the theme declares ${prefix} names`);
  return [...names];
}

function cn_preserves_theme_font_sizes_and_text_colours() {
  for (const size of themeUtilityNames("text")) {
    for (const colour of themeUtilityNames("color")) {
      const sizeClass = `text-${size}`;
      const colourClass = `text-${colour}`;
      assert.equal(cn(colourClass, sizeClass), `${colourClass} ${sizeClass}`);
      assert.equal(cn(sizeClass, colourClass), `${sizeClass} ${colourClass}`);
    }
  }
}

function cn_keeps_the_last_font_size() {
  const sizes = themeUtilityNames("text");
  for (const first of sizes) {
    for (const last of sizes) {
      assert.equal(cn(`text-${first}`, `text-${last}`), `text-${last}`);
    }
  }
}

function cn_keeps_the_last_text_colour() {
  const colours = themeUtilityNames("color");
  for (const first of colours) {
    for (const last of colours) {
      assert.equal(cn(`text-${first}`, `text-${last}`), `text-${last}`);
    }
  }
}

function cn_preserves_theme_shadows_and_shadow_colours() {
  for (const shadow of themeUtilityNames("shadow")) {
    const shadowClass = `shadow-${shadow}`;
    const colourClass = "shadow-primary";
    assert.equal(cn(colourClass, shadowClass), `${colourClass} ${shadowClass}`);
    assert.equal(cn(shadowClass, colourClass), `${shadowClass} ${colourClass}`);
  }
}

function cn_keeps_the_last_shadow_and_shadow_colour() {
  const shadows = themeUtilityNames("shadow");
  for (const first of shadows) {
    for (const last of shadows) {
      assert.equal(cn(`shadow-${first}`, `shadow-${last}`), `shadow-${last}`);
    }
  }
  assert.equal(cn("shadow-primary", "shadow-accent"), "shadow-accent");
  assert.equal(cn("shadow-accent", "shadow-primary"), "shadow-primary");
}

function cn_preserves_button_variant_colours_at_every_size() {
  const variantColours = {
    primary: "text-primary-foreground",
    secondary: "text-foreground",
    soft: "text-foreground",
    accent: "text-accent-foreground",
    destructive: "text-destructive-foreground",
    ghost: "text-foreground",
    link: "text-accent",
  } satisfies Record<NonNullable<ButtonProps["variant"]>, string>;
  const sizeClasses = {
    sm: "text-sm",
    md: "text-body-sm",
    lg: "text-base",
    icon: "size-11",
  } satisfies Record<NonNullable<ButtonProps["size"]>, string>;

  for (const variant of Object.keys(variantColours) as (keyof typeof variantColours)[]) {
    for (const size of Object.keys(sizeClasses) as (keyof typeof sizeClasses)[]) {
      const classes = cn(buttonVariants({ variant, size })).split(/\s+/u);
      assert.ok(classes.includes(variantColours[variant]), `${variant} ${size} keeps its text colour`);
      assert.ok(classes.includes(sizeClasses[size]), `${variant} ${size} keeps its size`);
    }
  }
}

test(cn_preserves_theme_font_sizes_and_text_colours);
test(cn_keeps_the_last_font_size);
test(cn_keeps_the_last_text_colour);
test(cn_preserves_theme_shadows_and_shadow_colours);
test(cn_keeps_the_last_shadow_and_shadow_colour);
test(cn_preserves_button_variant_colours_at_every_size);

test("the owner-supplied LayerX UI package is the visual contract", () => {
  const contract = human_design_tokens();
  assert.equal(contract.package, "@layerx/ui");
  assert.equal(contract.stylesheet, "@layerx/ui/styles.css");
  assert.ok(contract.styleFeatures.includes("borders"));
  assert.ok(contract.styleFeatures.includes("shadows"));
  assert.ok(contract.tokens.includes("--border"));
  assert.ok(contract.tokens.includes("--shadow-overlay"));
});

test("CommonJS formatting preserves token mappings and removes only deleted whitespace mappings", async () => {
  const source = "const value = 1;  \n";
  const formatted = formatCommonJs(source, "output.cjs");
  const map = new SourceMapGenerator({ file: "output.cjs" });
  map.addMapping({ generated: { line: 1, column: 6 },
    source: "input.ts", original: { line: 7, column: 8 }, name: "value" });
  map.addMapping({ generated: { line: 1, column: 18 },
    source: "input.ts", original: { line: 7, column: 20 } });
  const rawMap = JSON.parse(map.toString());
  rawMap.mappings = `A,${rawMap.mappings}`;
  const result = formatCommonJsMap(JSON.stringify(rawMap), source, formatted, "output.cjs");
  await SourceMapConsumer.with(result, null, (consumer) => {
    assert.deepEqual(consumer.originalPositionFor({ line: 1, column: 0 }),
      { source: null, line: null, column: null, name: null });
    assert.deepEqual(consumer.originalPositionFor({ line: 1, column: 6 }),
      { source: "input.ts", line: 7, column: 8, name: "value" });
    const columns: number[] = [];
    consumer.eachMapping((mapping) => columns.push(mapping.generatedColumn));
    assert.deepEqual(columns, [0, 6]);
  });
  map.addMapping({ generated: { line: 2, column: 1 },
    source: "input.ts", original: { line: 8, column: 1 } });
  assert.throws(() => formatCommonJsMap(map.toString(), source, formatted, "output.cjs"),
    /Invalid generated source-map location/u);
});

test("CommonJS formatting preserves literal whitespace and line positions", () => {
  const source = [
    'const word = "kept";  ',
    'const plain = `first  ',
    'second\t ',
    '${word}  ',
    '`; \t',
    'const tagged = String.raw`raw\\n  ',
    '${`nested  ',
    'value\t `}  ',
    '`;  ',
    'globalThis.result = { plain, tagged };  ',
    '// retained comment  ',
    '//# sourceMappingURL=output.cjs.map  ',
  ].join("\n");
  const formatted = formatCommonJs(source, "output.cjs");
  assert.equal(runInNewContext(`${formatted}\nJSON.stringify(result)`),
    runInNewContext(`${source}\nJSON.stringify(result)`));
  assert.equal(formatted.split("\n").length, source.split("\n").length);
  assert.equal(formatCommonJs(formatted, "output.cjs"), formatted);
  assert.ok(formatted.includes('const word = "kept";\n'));
  assert.ok(formatted.includes('first  \nsecond\t \n'));
  assert.ok(formatted.endsWith('//# sourceMappingURL=output.cjs.map'));
});

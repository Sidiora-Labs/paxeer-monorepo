// These functions run in the page, including before its scripts execute.
export function observePage() {
  const selector = (element) => {
    if (!element || element.nodeType !== Node.ELEMENT_NODE) return "document";
    if (element.id) return `#${CSS.escape(element.id)}`;
    const parts = [];
    while (element && element.nodeType === Node.ELEMENT_NODE) {
      const name = element.localName;
      const siblings = element.parentElement ? [...element.parentElement.children].filter((child) => child.localName === name) : [element];
      parts.unshift(`${name}:nth-of-type(${siblings.indexOf(element) + 1})`);
      const root = element.getRootNode();
      if (!element.parentElement && root.host) {
        parts.unshift(`${selector(root.host)} >>>`);
        break;
      }
      element = element.parentElement;
    }
    return parts.join(" > ").replace(" >>> > ", " >>> ");
  };
  const elements = (root = document) => [...root.querySelectorAll("*")].flatMap((element) => [element, ...(element.shadowRoot ? elements(element.shadowRoot) : [])]);
  const visible = (element) => {
    const style = getComputedStyle(element);
    return element.getClientRects().length > 0 && element.checkVisibility({ checkOpacity: true, checkVisibilityCSS: true }) && style.visibility === "visible" && style.display !== "none";
  };
  const measurements = { shifts: [], motions: [], selector, elements, visible };
  Object.defineProperty(window, "__uiCheck", { value: measurements });
  new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
      if (!entry.hadRecentInput) measurements.shifts.push({ value: entry.value, time: entry.startTime, sources: (entry.sources ?? []).map((source) => selector(source.node)) });
    }
  }).observe({ type: "layout-shift", buffered: true });
  const recordMotion = (event) => {
    const style = getComputedStyle(event.target, event.pseudoElement || null);
    const seconds = (value) => value.trim().endsWith("ms") ? parseFloat(value) : parseFloat(value) * 1000;
    const transition = event.type === "transitionrun";
    const names = (transition ? style.transitionProperty : style.animationName).split(",").map((name) => name.trim());
    const durations = (transition ? style.transitionDuration : style.animationDuration).split(",").map(seconds);
    const iterations = transition ? [1] : style.animationIterationCount.split(",").map((value) => value.trim() === "infinite" ? Infinity : Number(value));
    const name = transition ? event.propertyName : event.animationName;
    const index = Math.max(0, names.findIndex((entry) => entry === name || entry === "all"));
    const duration = durations[index % durations.length] * iterations[index % iterations.length];
    measurements.motions.push({ selector: selector(event.target), name, durationMs: Number.isFinite(duration) ? duration : "infinite" });
  };
  document.addEventListener("animationstart", recordMotion, true);
  document.addEventListener("transitionrun", recordMotion, true);
  window.addEventListener("unhandledrejection", (event) => {
    measurements.rejections ??= [];
    measurements.rejections.push(String(event.reason?.stack ?? event.reason));
  });
}

export function doubleText() {
  const { elements } = window.__uiCheck;
  const sizes = elements().map((element) => [element, getComputedStyle(element).fontSize,
    getComputedStyle(element, "::before").fontSize, getComputedStyle(element, "::after").fontSize]);
  const roots = new Set([document]);
  for (const [element, size, before, after] of sizes) {
    element.style.setProperty("font-size", `${parseFloat(size) * 2}px`, "important");
    element.style.setProperty("--ui-check-before-size", `${parseFloat(before) * 2}px`);
    element.style.setProperty("--ui-check-after-size", `${parseFloat(after) * 2}px`);
    roots.add(element.getRootNode());
  }
  for (const root of roots) {
    const stylesheet = document.createElement("style");
    stylesheet.textContent = "*::before{font-size:var(--ui-check-before-size)!important}*::after{font-size:var(--ui-check-after-size)!important}";
    (root === document ? document.head : root).append(stylesheet);
  }
}

export function measurePage({ limits, typeScale, doubled, reduced }) {
  const { elements, visible, selector, shifts, motions } = window.__uiCheck;
  const findings = [];
  const add = (kind, element, values) => findings.push({ kind, selector: typeof element === "string" ? element : selector(element), values });
  const width = window.innerWidth;
  if (document.documentElement.scrollWidth > width) add("sideways", "html", { width: document.documentElement.scrollWidth, viewport: width });
  const hasText = (element) => [...element.childNodes].some((node) => node.nodeType === Node.TEXT_NODE && node.textContent.trim()) || (element.matches("input,textarea,select") && (element.value || element.placeholder));
  for (const element of elements()) {
    if (!visible(element)) continue;
    const style = getComputedStyle(element);
    const rect = element.getBoundingClientRect();
    if (rect.width > width + 0.5 || rect.right > width + 0.5 || rect.left < -0.5) {
      add("sideways", element, { left: rect.left, right: rect.right, width: rect.width, viewport: width });
    }
    if (element.textContent.trim() && style.textOverflow !== "ellipsis") {
      const clipsX = ["hidden", "clip"].includes(style.overflowX);
      const clipsY = ["hidden", "clip"].includes(style.overflowY);
      if ((clipsX && element.scrollWidth > element.clientWidth + 1) || (clipsY && element.scrollHeight > element.clientHeight + 1)) {
        add("cut", element, { scrollWidth: element.scrollWidth, boxWidth: element.clientWidth, scrollHeight: element.scrollHeight, boxHeight: element.clientHeight, overflowX: style.overflowX, overflowY: style.overflowY });
      }
    }
    if (hasText(element)) {
      const fontSize = parseFloat(style.fontSize);
      const scaleSize = fontSize / (doubled ? 2 : 1);
      const matches = typeScale.some((step) => typeof step === "number" ? Math.abs(scaleSize - step) < 0.05 : scaleSize >= step.min - 0.05 && scaleSize <= step.max + 0.05);
      if (!matches) add("scale", element, { fontSize, scaleSize, typeScale });
    }
    for (const pseudo of ["::before", "::after"]) {
      const generated = getComputedStyle(element, pseudo);
      if (["none", "normal", '""'].includes(generated.content)) continue;
      const fontSize = parseFloat(generated.fontSize);
      const scaleSize = fontSize / (doubled ? 2 : 1);
      if (!typeScale.some((step) => typeof step === "number" ? Math.abs(scaleSize - step) < 0.05 : scaleSize >= step.min - 0.05 && scaleSize <= step.max + 0.05)) {
        add("scale", `${selector(element)}${pseudo}`, { fontSize, scaleSize, typeScale });
      }
    }
    const interactive = element.matches("button,a[href],input:not([type=hidden]),textarea,select,summary,[role=button],[role=link],[role=checkbox],[role=radio],[role=switch],[role=slider],[role=tab],[role=menuitem],[onclick],[tabindex]:not([tabindex='-1']),[contenteditable=true]");
    const inlineLink = element.matches("a[href]") && style.display === "inline" && element.parentElement && [...element.parentElement.childNodes].some((node) => node.nodeType === Node.TEXT_NODE && node.textContent.trim());
    if (width <= limits.touchWidth && interactive && !element.matches(":disabled,[aria-disabled=true]") && !inlineLink && (rect.width < limits.touch || rect.height < limits.touch)) {
      add("touch", element, { width: rect.width, height: rect.height, minimum: limits.touch });
    }
  }
  const shift = shifts.reduce((total, entry) => total + entry.value, 0);
  if (shift > limits.shift) add("shift", shifts.flatMap((entry) => entry.sources)[0] ?? "html", { score: shift, maximum: limits.shift, entries: shifts });
  if (reduced) {
    for (const animation of document.getAnimations()) {
      const timing = animation.effect?.getComputedTiming();
      const duration = timing?.activeDuration;
      if (duration > limits.motionMs && animation.playState === "running") {
        const target = animation.effect?.target;
        motions.push({ selector: selector(target), name: animation.animationName ?? animation.transitionProperty ?? "animation", durationMs: Number.isFinite(duration) ? duration : "infinite" });
      }
    }
    const seen = new Set();
    for (const motion of motions) {
      if (motion.durationMs !== "infinite" && motion.durationMs <= limits.motionMs) continue;
      const key = `${motion.selector}:${motion.name}`;
      if (!seen.has(key)) add("motion", motion.selector, { ...motion, maximumMs: limits.motionMs });
      seen.add(key);
    }
  }
  for (const message of window.__uiCheck.rejections ?? []) add("load", "document", { source: "unhandledrejection", message });
  return findings;
}

export function focusSnapshot() {
  const bodyTabindex = document.body.getAttribute("tabindex");
  document.body.tabIndex = -1;
  document.body.focus({ preventScroll: true });
  if (bodyTabindex === null) document.body.removeAttribute("tabindex");
  else document.body.setAttribute("tabindex", bodyTabindex);
  const { elements, visible, selector } = window.__uiCheck;
  const properties = ["outline-style", "outline-color", "outline-width", "outline-offset", "box-shadow", "background-color", "color", "border-top-color", "border-right-color", "border-bottom-color", "border-left-color", "border-top-width", "border-right-width", "border-bottom-width", "border-left-width", "text-decoration", "transform", "opacity", "content"];
  const appearance = (element) => {
    const targets = [element, ...element.querySelectorAll("*")];
    let parent = element.parentElement ?? element.getRootNode().host;
    while (parent) { targets.push(parent); parent = parent.parentElement ?? parent.getRootNode().host; }
    return JSON.stringify(targets.map((target) => ["", "::before", "::after"].map((pseudo) => {
      const style = getComputedStyle(target, pseudo || null);
      return properties.map((property) => style.getPropertyValue(property));
    })));
  };
  window.__uiCheck.appearance = appearance;
  window.__uiCheck.focusBefore = new Map(elements().filter(visible).map((element) => [element, appearance(element)]));
  window.__uiCheck.focusSelector = selector;
  return elements().filter((element) => visible(element) && element.tabIndex >= 0 && !element.matches(":disabled,[inert], [inert] *")).map(selector);
}

export function focusedElement() {
  let active = document.activeElement;
  while (active?.shadowRoot?.activeElement) active = active.shadowRoot.activeElement;
  if (!active || active === document.body || active === document.documentElement) return null;
  const { focusBefore, appearance, selector } = window.__uiCheck;
  window.__uiCheck.lastFocused = active;
  const rect = active.getBoundingClientRect();
  const baseline = focusBefore.get(active);
  return {
    selector: selector(active),
    changed: baseline !== undefined && baseline !== appearance(active),
    outside: rect.left < -0.5 || rect.top < -0.5 || rect.right > innerWidth + 0.5 || rect.bottom > innerHeight + 0.5,
    values: { left: rect.left, top: rect.top, right: rect.right, bottom: rect.bottom, viewportWidth: innerWidth, viewportHeight: innerHeight },
  };
}

export async function measureContrast(limits) {
  const plainValues = (value) => {
    if (Array.isArray(value)) return value.map(plainValues);
    if (value && typeof value === "object") {
      return Object.fromEntries(Object.entries(value)
        .filter(([, entry]) => entry !== undefined)
        .map(([key, entry]) => [key, plainValues(entry)]));
    }
    return value;
  };
  axe.configure({
    checks: [{
      id: "ui-control-contrast",
      evaluate: function (node, options) {
        if (!node.getClientRects().length || getComputedStyle(node).visibility !== "visible" || node.matches(":disabled,[aria-disabled=true]")) return true;
        const color = axe.commons.color;
        const parse = (value) => { const result = new color.Color(); result.parseString(value); return result; };
        const style = getComputedStyle(node);
        const parent = node.parentElement;
        const background = color.getBackgroundColor(parent ?? node);
        const ownBackground = color.getBackgroundColor(node);
        const parts = [];
        for (const side of ["Top", "Right", "Bottom", "Left"]) {
          if (parseFloat(style[`border${side}Width`]) > 0 && style[`border${side}Style`] !== "none") {
            const foreground = parse(style[`border${side}Color`]);
            if (foreground.alpha > 0) parts.push({ part: `border${side}`, ratio: color.getContrast(background, foreground) });
          }
        }
        if (!parts.length && parse(style.backgroundColor).alpha > 0 && parent && style.backgroundColor !== getComputedStyle(parent).backgroundColor) {
          parts.push({ part: "background", ratio: color.getContrast(background, parse(style.backgroundColor)) });
        }
        for (const icon of node.querySelectorAll("svg,svg *")) {
          const iconStyle = getComputedStyle(icon);
          if (!icon.getClientRects().length) continue;
          for (const property of ["fill", "stroke"]) {
            if (iconStyle[property] === "none" || iconStyle[property].startsWith("url(")) continue;
            const foreground = parse(iconStyle[property]);
            if (foreground.alpha > 0) parts.push({ part: property, ratio: color.getContrast(ownBackground, foreground) });
          }
        }
        const failing = parts.filter((part) => part.ratio !== null && part.ratio < options.minimum);
        this.data({ minimum: options.minimum, parts });
        return !failing.length;
      },
      options: { minimum: limits.controlContrast },
    }],
    rules: [{ id: "ui-control-contrast", selector: "button,input:not([type=hidden]),textarea,select,[role=button],[role=checkbox],[role=radio],[role=switch],[role=slider]", any: ["ui-control-contrast"], tags: ["wcag1411"] }],
  });
  const result = await axe.run(document, {
    runOnly: { type: "rule", values: ["color-contrast", "ui-control-contrast"] },
    rules: { "color-contrast": { options: { contrastRatio: { normal: { expected: limits.textContrast }, large: { expected: limits.largeContrast } } } } },
  });
  const findings = [];
  for (const violation of [...result.violations, ...result.incomplete]) {
    for (const node of violation.nodes) {
      const checks = [...node.any, ...node.all, ...node.none];
      const measured = checks.map((check) => check.data === undefined ? null : plainValues(check.data)).filter(Boolean);
      const measuredFailure = measured.some((data) => typeof data.contrastRatio === "number" && data.contrastRatio < parseFloat(data.expectedContrastRatio));
      if (result.violations.includes(violation) || (violation.id === "ui-control-contrast" && measuredFailure)) {
        findings.push({ kind: "contrast", selector: node.target.flat(Infinity).join(" >>> "), values: { rule: violation.id, measurements: measured, message: node.failureSummary } });
      }
    }
  }
  return findings;
}

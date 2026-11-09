// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/room/lookup.js (MIT; see LICENSE-ENGINE).
// Prompt-lookup drafts: when the text being written repeats something already in the context
// (a quote, a name, code being edited, a list being continued), the tokens that followed the
// same n-gram earlier are a free guess for what comes next. The trunk verifies them exactly like
// the draft head's guesses, so a wrong guess costs nothing but the lap it rides on.

// ctx: token ids so far (conversation + answer, ending with the token about to be verified).
// Returns up to k tokens that followed the most recent earlier occurrence of ctx's last n tokens
// (n from maxN down to minN), or [] when nothing matches.
export function lookupDrafts(ctx, k, { maxN = 4, minN = 2, window = 16384 } = {}) {   // the whole context: code edits copy from anywhere in it
  const L = ctx.length;
  const lo = Math.max(0, L - window);
  for (let n = Math.min(maxN, L - 1); n >= minN; n--) {
    const tail = L - n;
    for (let i = tail - 1; i >= lo; i--) {   // candidate start of an earlier copy of the last n tokens
      let j = 0;
      while (j < n && ctx[i + j] === ctx[tail + j]) j++;
      if (j < n) continue;
      const from = i + n, to = Math.min(from + k, L);
      if (to > from) return ctx.slice(from, to);
    }
  }
  return [];
}

// Dense verify frames (DenseEngine.specStepDrafts: 1..8 columns with `spec`) need workers whose
// dense engine takes any column count; a worker from before that assumed 4 columns and breaks on
// them. Every device advertises what it handles in its hello meta (dspec); the host speculates on a
// dense model only while every layer-holding device in the chain advertises it, else plain laps.
export const DENSE_SPEC_V = 1;
export function chainDenseSpec(metas) {
  return metas.length > 0 && metas.every((m) => (m?.dspec | 0) >= DENSE_SPEC_V);
}
// The lookup drafts for one dense step: none unless the chain handles dense verify frames; up to 3
// until a run was accepted in full (`full`), then up to kMax.
export function denseLookupDrafts(chainMetas, ctx, kMax, { full = false } = {}) {
  if (!chainDenseSpec(chainMetas) || kMax < 1) return [];
  return lookupDrafts(ctx, full ? kMax : Math.min(3, kMax));
}

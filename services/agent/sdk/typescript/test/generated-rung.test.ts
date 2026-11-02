import assert from "node:assert/strict";
import { test } from "node:test";

import { Rung, type SubmissionState } from "../src/generated/client.js";

function generated_submission_state_names_the_rungs(): void {
  const variants = {
    Submitted: true,
    Receipted: true,
    Committed: true,
    Reverted: true,
  } satisfies Record<SubmissionState["kind"], true>;
  const states: SubmissionState[] = [
    { kind: "Submitted", txHash: "0x1234" },
    { kind: "Receipted" },
    { kind: "Committed" },
    { kind: "Reverted", revertReason: "execution reverted" },
  ];

  assert.deepEqual(states.map((state) => state.kind), Object.keys(variants));
  for (const state of states) {
    switch (state.kind) {
      case "Submitted":
        assert.equal(state.txHash, "0x1234");
        break;
      case "Receipted":
      case "Committed":
        break;
      case "Reverted":
        assert.equal(state.revertReason, "execution reverted");
        break;
      default: {
        const unexpected: never = state;
        assert.fail(`unexpected submission state: ${JSON.stringify(unexpected)}`);
      }
    }
  }

  assert.deepEqual(
    Object.keys(Rung).filter((name) => Number.isNaN(Number(name))),
    ["Submitted", "Receipted", "Committed"],
  );
  assert.deepEqual([Rung.Submitted, Rung.Receipted, Rung.Committed], [0, 1, 2]);
}

test(generated_submission_state_names_the_rungs);

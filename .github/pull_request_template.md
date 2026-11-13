## Summary

Describe the change, the affected trust boundary, and why it is necessary.

## Specification

- Linked spec task or requirement:
- Compatibility impact on wire encoding, result codes, state roots, storage, or contract ABI:

## Test evidence

List every command run, the verified revision, and its exact outcome. Explain any gate not run.
Include only reviewed technical evidence; never paste credentials or private logs.

```text
revision:
command:
exit code:
log or CI run link:
```

## Checklist

- [ ] I changed KVX/source documents rather than generated specification mirrors.
- [ ] I preserved canonical encoding, deterministic replay, and the merged chain-125 paxd ledger's committed-state authority and native bank invariants.
- [ ] I added real positive, negative, and adversarial coverage appropriate to the change.
- [ ] I introduced no credentials, local artifacts, private infrastructure identifiers, mocks, stubs, or placeholders.
- [ ] I am not representing local verification as production certification or deployment authorization.
- [ ] **DCO:** I certify that each commit is signed off (`Signed-off-by:`) under the Developer Certificate of Origin.
- [ ] **No weakening:** I did not relax an assertion, loosen a bound, widen a type, add a silent fallback, skip or delete a test, or disable a check to make anything pass.

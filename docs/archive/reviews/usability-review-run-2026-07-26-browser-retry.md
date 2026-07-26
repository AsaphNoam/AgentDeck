# Usability Review Run — 2026-07-26 browser retry

**Scope:** resume the post-fix browser matrix from the blocked J5 variants, then begin J7 using
fresh loopback listeners and the retained review fixtures. No product code or specifications were
changed.

## Results

| Journey | Result | Evidence |
|---|---|---|
| J5 group release confirmation | **INCONCLUSIVE** | The in-app browser completed the native-confirm interaction and the two displayed cards became stopped. That listener was discovered to belong to an earlier review home, so it is not valid evidence for this rerun. |
| J5 restart/delete variants | **BLOCKED(browser fixture contamination and tab loss)** | A new J5 listener exposed the expected Layout A/B/C fixture. Earlier browser pages sharing the old port repeatedly saved their retained default layout into that home after restart, so the persistence/delete result is no longer isolated. |
| J7 stopped-agent, transcript, and Archive entry | **PARTIAL PASS** | An isolated J7 home on port 4321 rendered both lived-in stopped agents; its transcript page rendered, and Archive listed both inactive sessions. The browser tab disappeared immediately before the Archive-row action that would exercise Resume/switch. |
| J8–J14 | **BLOCKED(browser tab loss)** | The in-app browser repeatedly reported no tab after a page transition. No later journey was inferred from tests or source evidence. |

## Browser limitation

The previous native-confirm stall did not recur: the confirmation click completed. The current
blocker is different: after a successful render or transition, the in-app browser's tab is no
longer available to the automation API. Reacquiring a fresh tab works for one initial render, but
the next interaction loses it again. The review was stopped rather than switching to a non-browser
tool or treating automated regression coverage as real-browser evidence.

## Conclusion

None of the seven repaired review findings can yet be marked as exercised end-to-end in a real
browser. The J7 entry and Archive list are newly observed browser surfaces, but they do not reach
the repaired Resume, Archive-header/paging, annotation, pipeline-link, or validation paths.

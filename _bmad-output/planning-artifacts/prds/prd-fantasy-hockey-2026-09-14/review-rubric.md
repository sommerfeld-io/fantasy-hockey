# PRD Quality Review — Fantasy Hockey

## Overall verdict

A well-calibrated, unusually rigorous PRD for its stakes: FRs are near-uniformly backed by testable consequences, deferrals are named honestly (concurrent-write risk, `fantasy-hockey.yml` schema, email delivery), and the brownfield lineage (click-dummy vs. new-for-rebuild) is tracked meticulously throughout §4. The one real hole is that nothing in the document — not Open Questions, not Assumptions, not Non-Goals — acknowledges that this is a full rebuild aimed at a hard external deadline (NHL season start) roughly a month out from the PRD date, with no phasing or fallback discussed. A couple of terminology-precision nits (Player vs. player; new scoring values presented without rationale) round out the findings, but none of them threaten the document's core usefulness as a build spec.

## Decision-readiness — adequate

The PRD generally states decisions as decisions rather than hedging them: deadline enforcement has "no grace period and no override for any Player, including whoever made the pick" (§4.2, FR-8); standings ties get "no tiebreaker of any kind" (FR-23); the concurrent-write risk on `fantasy-hockey.yml` is explicitly "accepted as low-risk with only 3 trusted participants" rather than smoothed over (§8, "Resolved during PRD review"). Open Questions (§8) are genuinely open — the email-delivery dependency and the YAML schema-ownership question are both real, deferred to `bmad-architecture`, not answered in the next sentence.

What's missing is any acknowledgment of build-timeline risk. The PRD is dated 2026-09-14 and targets "NHL 2026–27" (§1, §7 SM-1) — before-the-season Prediction sets (Cup champion, Presidents' Trophy, Division picks, Awards; FR-13–17) must close before puck drop, which is roughly a month out. This is a from-scratch rebuild — new auth, new scoring engine, new persistence, new reminder emails (§4.9's own framing: "New for this rebuild") — being built solo by "the builder (also the sole PM, architect, and one of the three players)" (§0). Nowhere — not §8 Open Questions, not §9 Assumptions, not §5/§6 scope — is there any discussion of whether this is achievable in time, or what happens if it isn't (e.g., falling back to the Excel workbook for before-the-season predictions and cutting over for playoffs only). A reader pushing back with "can this ship before the season starts, and what's the fallback?" would find the objection neither raised nor dodged — just absent.

### Findings

- **high** No build-timeline risk or fallback plan (whole document) — The PRD targets a hard external deadline (NHL season start, ~1 month after the PRD date) for a full solo rebuild, but no section discusses feasibility risk or a fallback if v1 isn't ready in time (e.g., partial cutover, temporary Excel fallback for before-the-season picks). *Fix:* Add an Open Question or `[NOTE FOR PM]` naming the timeline risk and the fallback if the before-the-season Prediction sets aren't ready by season start.
- **medium** Scoring point values presented without rationale (§4.6, FR-24) — The FR-24 table (award=5, division winner=15, Presidents' Trophy=20, Round 1 series=15/25, etc.) is explicitly new for this rebuild ("the scoring rules themselves are new for this rebuild — the click-dummy explicitly left them open," §4.6 description), yet no rationale is given and there's no signal that these are provisional/best-guess values worth revisiting after a season. *Fix:* Add a short note (or `[ASSUMPTION]`) that the point values are the PM's own initial calibration and may be revisited after season 1.

## Substance over theater — strong

No furniture found. The Vision (§1) is anchored to a specific, non-transferable fact — "a manually maintained Excel workbook... had already caused a real, uncorrected scoring error in a past season" — that could not swap into another PRD unchanged. The single "Player" role, applied consistently to three named individuals (Basti, Sadl, Tobbi) who each drive specific UJs, is the opposite of persona theater. There's no differentiation/competitive section (correctly absent for a private 3-person tool) and no generic-boilerplate NFR language ("must be scalable," "must be secure") anywhere — every behavioral constraint found (session timeout, code expiry, dark-theme-only) is a specific, testable number or rule.

## Strategic coherence — strong

The thesis is explicit and load-bearing: get "the player-facing loop — predict, wait, get scored, compare, gloat — right, reliably" (§1), specifically to fix the three named failure modes of the old spreadsheet (scoring errors, deadline confusion, name-typo scoring gaps). SM-C1 restates those exact three failure modes as the counter-metric boundary, which is a tight thesis-to-metric link. SM-1 measures actual behavior change (full-season use without reverting to Excel) rather than an activity proxy like DAU/MAU — a good sign the metric validates the thesis rather than just measuring usage.

### Findings

- **low** SM-1 is a single lagging, end-of-season signal (§7) — There's no earlier checkpoint (e.g., "all three Players complete the before-the-season sets without reverting to Excel") to catch failure mid-flight; success/failure is only knowable after the value window has largely passed. *Fix:* Consider a leading indicator tied to the first Prediction set's deadline, even if informal.

## Done-ness clarity — strong

This is the PRD's best dimension. Nearly every FR carries a "Consequences (testable)" block with concrete, verifiable statements: "A code is valid for 10 minutes or until used once, whichever comes first" (FR-2); "A session ends after 30 minutes of inactivity" (FR-3); "Each Conference must total exactly 8 teams... in a 4/4 or 5/3 split" (FR-15). No instances of "handles gracefully," "reasonable performance," or "user-friendly" were found anywhere in §4. Where an FR lacks a Consequences block (e.g., FR-13, FR-14, FR-16), the FR statement itself is already concrete enough to be testable, consistent with the rubric's allowance for that.

### Findings

- **low** FR-33 doesn't say how the NHL player list is maintained (§4.9) — "a working list of NHL players for award-finalist autocomplete" is available "everywhere it's used," but unlike every other `fantasy-hockey.yml`-sourced datum in this PRD (deadlines, matchups, results — each explicitly "edited directly... out of band"), FR-33 never says who updates this list or how often (static per season vs. updated for mid-season trades/injuries). *Fix:* State the maintenance model explicitly, even if it's the same out-of-band pattern as everything else in the file.

## Scope honesty — adequate

§5 Non-Goals is concrete and specific (8 bullets, each naming a real capability rather than a vague category), and §6.2 correctly distinguishes permanent exclusions ("Any admin/management UI... — permanently out of scope, not just deferred") from deferred ones (season-selector, deferred "until a second season's history exists"). Both `[ASSUMPTION]` tags are load-bearing, not filler, and both round-trip cleanly into §9 (see Mechanical notes). The concurrent-write deferral in the addendum is de-scoped honestly, with an explicit revisit trigger ("if the file ever needs to support genuinely concurrent multi-writer scenarios").

The gap is the same one noted under Decision-readiness: the build-timeline risk against the season-start deadline is an omission the reader must infer rather than one the PRD surfaces — it doesn't appear as a Non-Goal, an `[ASSUMPTION]`, or an Open Question, despite being arguably the single biggest practical risk to the Vision shipping at all. Open-items density (2 Open Questions + 2 Assumptions) is otherwise appropriately low for hobby/internal stakes, per the rubric's own stakes-scaling — this isn't a case of high density signaling trouble, it's a case of one specific, consequential omission.

## Downstream usability — strong

Chain-top usage is explicit (§0: "for the downstream BMad workflows that consume it next — UX, architecture, epics/stories"), and the PRD is built for it: the Glossary (§3) is comprehensive and cross-references resolve by ID/term rather than "see above" (e.g., "except a Series pick's winner and game count... (see FR-19)"). FR IDs (FR-1–FR-34) are contiguous with no gaps or duplicates. All six UJs (UJ-1–UJ-6) have named protagonists and are each explicitly realized by at least one feature ("Realizes UJ-2," §4.1; "Realizes UJ-6," FR-20; etc.), with no floating UJs and no orphaned "Realizes" references.

### Findings

- **medium** "Player" vs. "player" terminology collision (§3 Glossary; FR-17; FR-33) — The Glossary defines **Player** (capitalized) as "the app's only role/actor" — one of the three pool participants. But FR-17 ("autocomplete against the season's known player list") and FR-33 ("a working list of NHL players for award-finalist autocomplete") use lowercase "player" to mean an NHL athlete — a completely different, much larger set. The Glossary never defines an "NHL player" term to disambiguate, which risks a downstream extraction (architecture or story-writing) misreading "player list" as a list of the three pool Players. *Fix:* Add a distinct Glossary term (e.g., "NHL Player" or "Athlete") for the FR-17/FR-33 sense, or rephrase those FRs to avoid the bare word "player."

## Shape fit — strong

This is a hobby/solo-adjacent, brownfield, chain-top PRD, and it's shaped accordingly on every count the rubric names. UJs are deliberately light ("single-line JTBD-style journeys," §2.3) rather than over-formalized for a single-role tool with three named actors — proportionate, not theater. Brownfield sourcing is unusually disciplined: every feature description states plainly whether it's "Prototype-validated" (§4.3, §4.4, §4.7, §4.8) or "New for this rebuild" (§4.5, §4.6, §4.9), which is exactly the existing-vs-new distinction the rubric calls load-bearing for brownfield PRDs. No differentiation/competitive section was forced in despite the template pressure that often produces one, and no admin/commissioner role was added just because such tools "usually" have one — §5 closes that off explicitly.

## Mechanical notes

- **ID continuity**: FR-1 through FR-34 are contiguous, unique, and every cross-reference checked (FR-9↔FR-19, FR-10↔FR-20, FR-11↔FR-24, FR-17↔FR-33) resolves correctly. UJ-1 through UJ-6 are each referenced by exactly one feature's "Realizes UJ-n" and no UJ is orphaned.
- **Assumptions Index roundtrip**: Clean. Two inline `[ASSUMPTION]` tags (§6.1, §7) and both appear indexed in §9 with matching content; no index entries lack an inline source.
- **Glossary drift**: One minor case inconsistency — FR-6's heading reads "Browse prediction sets" (lowercase) while its own body text and the Glossary use "Prediction sets" (capitalized) one line later. Cosmetic, but worth a pass. See also the Player/player collision under Downstream usability, which is a substantive instance of the same category.
- **UJ protagonist naming**: All six UJs name a protagonist inline (Basti, Sadl, Tobbi, rotated) and carry enough context to stand alone — no floating UJs.
- **Required sections**: All sections expected for a chain-top, hobby-stakes, brownfield PRD are present (Vision, Target User/JTBD/UJs, Glossary, Features/FRs, Non-Goals, MVP Scope, Success Metrics, Open Questions, Assumptions Index); no unexplained gaps.

# Spine Pair Review — Fantasy Hockey

## Overall verdict

The pair is a disciplined, faithful distillation of the click-dummy with accurate PRD traceability (every FR citation checked resolves correctly) and unusually honest disclosure of its own gaps (FR-22 deferral, Leaderboard/Player awards terminology divergence are both logged, not hidden). Nothing found here breaks source-extraction outright. But a downstream consumer would still hit real potholes: DESIGN.md has no visual spec at all for input/dropdown fields — used by a third of EXPERIENCE.md's components — and a handful of load-bearing UX decisions (invalid-finalist-name treatment, Compare's default state) are simply uncommitted. One more click-dummy-vs-PRD terminology divergence exists that the spine didn't catch and log alongside the one it did.

## 1. Flow coverage — adequate

Checked all 6 UJs (PRD §2.3) against EXPERIENCE.md's 5 Key Flows.

| UJ | Protagonist | Key Flow | Steps | Climax | Failure path |
|----|-------------|----------|-------|--------|---------------|
| UJ-1 (fill picks, blank scores zero) | Basti | Flow 2 | yes | yes | not applicable-ish, see finding |
| UJ-2 (login via code) | Sadl | Flow 1 | yes | yes | yes |
| UJ-3 (Compare mid-playoffs) | Tobbi | Flow 3 | yes | yes | n/a |
| UJ-4 (reminder email) | Basti | **none** | — | — | — |
| UJ-5 (Standings/Leaderboard live) | Sadl | Flow 4 | yes | yes | n/a |
| UJ-6 (admin unlocks round) | pool runner → reframed as Basti | Flow 5 | yes | yes | n/a |

### Findings

- **Medium** UJ-4 has no Key Flow — FR-22 (reminder email) is explicitly deferred, so this is disclosed three times (frontmatter note, "Deferred" section, "Open Items"), but the mechanical UJ→Flow mapping is still broken for a consumer scanning Key Flows alone (EXPERIENCE.md lines 14, 37–39, 145). *Fix:* none needed here if the planned PRD follow-up removes FR-22/UJ-4 from MVP; otherwise this spine needs a Flow once FR-22 gets a UI.
- **Low** FR-9/FR-19's testable rule that a Series pick's winner and game count "save together as one unit" (unlike every other field, which saves independently) isn't stated in the "Playoff round series" row of Component Patterns (EXPERIENCE.md line 67) — only the equal-visual-weight rule is carried over. *Fix:* add one clause restating the atomic-save behavior.
- **Low** Flow 2 (fill in picks) has no failure/edge path even though a real one exists and is documented elsewhere (Division-cap validation blocking submit, State Patterns line 83). Acceptable since it's covered elsewhere, but the flow itself glosses over it.

## 2. Token completeness — strong

Extracted all frontmatter tokens (17 colors, typography.body/numeric/scale/weight/case, rounded.lg/xl/full, spacing.gutter/card-padding, 6 `components` entries) and every `{path.to.token}` reference inside the `components` block. All resolve; every color has a hex value (no critical misses).

### Findings

- **Low** The Table component's "Total column ... visually set apart with a left border and tint" (DESIGN.md line 120) never names which token supplies the tint — no `{colors.X}` reference, no hex. Minor, but it's the one visual value in the Components section left unsourced.

## 3. Component coverage — thin

Extracted every component name used in DESIGN.md.Components (8: Status pill, Set row, Chip, Number button, Card/container, Primary button, Rank badge, Table) and EXPERIENCE.md.Component Patterns (12 rows, including 3 Login/form rows).

### Findings

- **Medium** No DESIGN.md Components entry exists for input/dropdown/text-field styling at all, despite three EXPERIENCE.md rows depending on it: Login — email/code fields, Single-team pick dropdowns, Division picks — division winners dropdowns, and Player awards text inputs (EXPERIENCE.md lines 59–66; DESIGN.md lines 111–120 have no such bullet). The values exist — they're baked directly into `mockups/login-email.html`/`login-code.html`'s CSS (`raised` fill, `border`, `lg` radius) and in `imports/clickdummy-ui-ux.md` §6.1/§6.3 — but never committed to DESIGN.md itself. *Fix:* add an "Input / dropdown field" bullet to DESIGN.md.Components (and ideally a `components.input` frontmatter entry) sourced from the mockup CSS already in the repo.
- **Low** Several DESIGN.md components (Chip, Number button, Rank badge, Card/container, Primary button) have no identically-named EXPERIENCE.md row — their behavior is folded into other rows instead (Division picks, Playoff round series, Leaderboard table, Prediction sheet). Functionally covered, but the two tables don't name-match 1:1, so side-by-side cross-referencing takes extra work.

## 4. State coverage — adequate

Walked all 6 IA surfaces (Login-email, Login-code, Predict, Prediction sheet, Leaderboard, Compare) against the 10-row State Patterns table.

### Findings

- **Medium** Player awards' "a name that doesn't match the list is rejected, not silently accepted" (EXPERIENCE.md line 66) has no State Patterns row — unlike Division picks' invalid state, which gets an explicit one (line 83). How the rejection surfaces (inline error text? red border? nothing visible?) is a load-bearing decision left uncommitted.
- **Medium** Compare has no defined default/initial-selection state. Component Patterns says "Selecting a chip swaps the comparison table below" (line 69) but never states what — if anything — renders before any chip is tapped on first visit. A downstream consumer can't build Compare's first paint from this spine alone.
- **Low** Neither Login screen defines a client-side validation-error state (malformed/empty email) or an in-flight/loading state for either submit action.

## 5. Visual reference coverage — adequate

Listed all files in `mockups/` (2) and `imports/` (2 markdown + the full `faceoff-pool-source`/`faceoff-pool-dist` trees).

### Findings

- **Medium** `imports/clickdummy-requirements.md` is cited only in EXPERIENCE.md's frontmatter `sources:` list (line 7) and never pointed to inline anywhere in either spine's body — an orphan by the "link inline at the relevant section" standard, even though `.memlog.md` confirms it drove the scope-reconciliation work.
- **Low** `imports/faceoff-pool-source` / `imports/faceoff-pool-dist` are cited only at directory level (IA section, line 35); the one file that actually encodes almost all click-dummy behavior, `src/App.jsx` (807 lines), is never named directly anywhere — only `package.json` gets a specific inline citation (Foundation, line 18). A downstream reader verifying exact behavior (e.g. the chip-cap logic) has to locate App.jsx unassisted.
- Both `mockups/` files are well-cited at every relevant section (IA, Component Patterns, Key Flows), and "spine wins on conflict" is stated exactly once (EXPERIENCE.md line 35). No unresolved paths found.

## 6. Bloat & overspecification — strong

No findings. Both documents track the click-dummy's actual scope closely; the Do's/Don'ts tables restate prose rules, matching the same pattern in the exemplar (`design-example-mobile.md`), not padding.

## 7. Inheritance discipline — adequate

Checked frontmatter `sources:` resolution, FR-citation accuracy, terminology identity, and cross-file token-name resolution.

### Findings

- **Medium** A second, undisclosed click-dummy-vs-PRD terminology divergence exists beyond the one the spine calls out. EXPERIENCE.md's Component Patterns table (line 63) names a set "Playoffs Cup pick," matching the click-dummy's own label (`imports/clickdummy-ui-ux.md` line 87), while the PRD Glossary and FR-18 both say "Playoffs Cup **re-**pick" (prd.md lines 62, 194–195). This is the same kind of deliberate-preservation choice as the disclosed Leaderboard/Player awards divergence, but it isn't logged in the frontmatter note (line 14) or "Open Items" (line 144) alongside it. *Fix:* add "Playoffs Cup pick" → "Playoffs Cup re-pick" to the same disclosure so the planned `bmad-prd` Update catches it too.
- **Low** DESIGN.md's Colors prose names four compound tokens in camelCase (`iceDeep`, `greenBtn`, `greenBtnBorder`, `borderSoft` — lines 88–89, 120) that are defined in the frontmatter under kebab-case keys (`ice-deep`, `green-btn`, `green-btn-border`, `border-soft` — lines 15–16, 19), inherited verbatim from the click-dummy's own camelCase token table without being normalized. Not a broken `{}` reference (these are plain backticked mentions, not cross-ref syntax), but the name doesn't match its own frontmatter definition — confusing for anyone trying to look one up.
- **Low** EXPERIENCE.md's Accessibility Floor asserts specific tap-target pixel sizes (32px, 36px — line 96) with no corresponding DESIGN.md token to source them from; presumably measured off the click-dummy's CSS but not traceable to anything committed in either spine.
- No misses: all `sources:` paths resolve to real files; spot-checked FR citations (FR-1, FR-2, FR-3, FR-9, FR-18/19, FR-20, FR-22) all match `prd.md`'s actual numbering and content; the deliberate Leaderboard/Player awards divergence is exactly as logged.

## 8. Shape fit — strong

DESIGN.md's 8 sections appear in the exact canonical order from `design-md-spec.md` (Brand & Style → Colors → Typography → Layout & Spacing → Elevation & Depth → Shapes → Components → Do's and Don'ts). EXPERIENCE.md carries every section present in the mobile exemplar (Foundation, IA, Voice and Tone, Component Patterns, State Patterns, Interaction Primitives, Accessibility Floor, Key Flows); "Deferred" and "Open Items" are additions beyond the exemplar but are load-bearing BMad-pattern sections (not filler), and "Inspiration & Anti-patterns" is reasonably omitted given this project is a near-literal clone rather than a from-scratch design with lifted/rejected patterns to document.

## Mechanical notes

- Both documents are still `status: draft`, consistent with `.memlog.md`'s note that a "Reviewer Gate" run precedes finalize.
- No broken cross-references found: every frontmatter `sources:` path and every inline mockup/import link resolves to a real file in the repo.
- EXPERIENCE.md cites PRD FR numbers throughout (FR-1, FR-2, FR-3, FR-20, FR-22, etc.) but never cites UJ numbers directly, even though its Key Flows are explicitly UJ-shaped — traceability to UJs is indirect, via the PRD's own FR→UJ mapping (e.g. PRD §4.1 states "Realizes UJ-2," but EXPERIENCE.md's Login rows only say "Realizes PRD FR-1"/"FR-2").
- Naming inconsistencies are detailed under §7 above (camelCase/kebab-case token mismatch in DESIGN.md; "Playoffs Cup pick" vs "Playoffs Cup re-pick" across EXPERIENCE.md and the PRD).
- Frontmatter completeness: both files carry `name`/`status`; EXPERIENCE.md additionally carries `sources`/`updated` (matching the exemplar's shape); DESIGN.md's lack of a `sources` field matches the exemplar's own shape (`design-example-mobile.md` also omits it) and is not a defect.

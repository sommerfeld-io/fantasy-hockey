---
title: 'Keep header and bottom nav visible on iOS Safari'
type: 'bugfix'
created: '2026-10-19'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** On iOS Safari the app shell (`.shell`) is `height: 100vh`, the large viewport, so while the address bar is showing the frame is taller than the visible area and the bottom nav sits under the browser chrome. Scrolling to reveal it scrolls the whole page and pushes the header out of view.

**Approach:** Size the shell with dynamic viewport units (`100dvh`, `100vh` fallback) so it always equals the visible area, let only `.shell-content` scroll, and respect the iPhone safe area (`viewport-fit=cover`, bottom inset on the nav). Switch the other full-height rules to `dvh` as well. The user explicitly skipped the acceptance test (visual, not testable in GoDog).

</frozen-after-approval>

## Implementation Notes

- `styles.css`: `.shell` gets `height: 100dvh` (100vh fallback kept first) and `overflow: hidden`; `.shell-content` gets `overscroll-behavior: contain`; `.shell-nav` gets `padding-bottom: env(safe-area-inset-bottom)`; `.screen` and `.sheet` min-height gets a `dvh` line.
- `shell.html`: viewport meta gains `viewport-fit=cover`. Login/sheet templates left alone (no pinned nav).
- Tests (red first, then green): stylesheet and viewport-meta assertions in `web_test.go` / `shell_test.go`. Acceptance test skipped at the user's request.
- Verified: `go test ./internal/web/`, `task go:run` (built, served the dvh stylesheet). Still needs a manual check on a real iPhone.

## Review Triage Log

- Substring test matched `min-height` too -- medium -- fixed by asserting the newline-anchored declarations.
- Top/side safe-area insets -- low -- rejected: no top overlap in Safari portrait; shell is centered at 430px in landscape.
- Other templates' viewport meta -- false -- they have no pinned nav.
- Nav box-sizing/height -- false -- global border-box, height auto.
- `overflow: hidden` clipping -- low -- no fixed/absolute children rely on page scroll.
- `overscroll-behavior` on iOS < 16 -- low -- rejected, degrades to prior behaviour.

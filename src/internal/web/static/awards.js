/*
  awards.js drives the "awards" pick sheet's finalist autocomplete widget
  (AD-19/AD-10): each finalist slot pairs a visible text <input> with a
  hidden slug <input>. Typing filters the slot's own position-scoped
  embedded {"id","label"} list (case-insensitive substring match on label)
  and renders a suggestion list; picking a suggestion fills the hidden slug
  and the visible text. Any edit to the text clears the hidden slug
  immediately, so a stale slug can never survive an edited-but-unconfirmed
  name; the shared input.error goal-border convention only lights up once
  the field is left (blurred) still unresolved, not on every keystroke.
  This is the app's first true
  autocomplete widget, and it is cosmetic-immediacy only (AD-10's "the
  server independently re-validates and enforces every rule on submit
  regardless of what the client allowed") - handleAwardsSubmit still
  rejects an unresolved or wrong-position slug server-side regardless of
  what this script allowed, so a disabled-JS or bypassing client still
  can't persist an invalid pick. AD-10 also caps this file's own
  responsibility to exactly this one widget - no submit-button
  disabling/enabling logic lives here, unlike divisions.js's own cap
  bookkeeping.
*/

const MAX_SUGGESTIONS = 8;

// optionsForPosition parses position's own embedded {"id","label"} script
// tag (one per position, shared across every slot needing it) - re-parsed
// on every keystroke rather than cached, since this list is small (a
// season's roster) and never changes after page load.
function optionsForPosition(position) {
  const script = document.getElementById("award-options-" + position);
  if (!script) {
    return [];
  }
  try {
    return JSON.parse(script.textContent);
  } catch (err) {
    return [];
  }
}

// closeSuggestions removes slot's own rendered suggestion list, if any.
function closeSuggestions(slot) {
  const list = slot.querySelector(".finalist-suggestions");
  if (list) {
    list.remove();
  }
}

// updateGroupCheck toggles group's own green-check visibility based on
// whether every one of its 3 slots currently carries a resolved (non-empty)
// slug - a live, cosmetic-only mirror of the server-rendered .Filled state.
function updateGroupCheck(group) {
  if (!group) {
    return;
  }
  const slugs = Array.from(group.querySelectorAll(".finalist-slug"));
  const filled = slugs.length === 3 && slugs.every((input) => input.value !== "");
  group.classList.toggle("award-group--filled", filled);
}

// clearSlug empties slot's hidden slug input - called on every keystroke so
// a stale slug never survives an edited-but-unconfirmed name. It does not
// itself toggle the goal-border error class: that only happens once the
// field is left unresolved (markUnresolvedOnBlur), matching the AC's "when
// I try to save that field" timing rather than flashing on mid-search.
function clearSlug(slugInput) {
  slugInput.value = "";
}

// markUnresolvedOnBlur toggles the shared input.error goal-border
// convention once the user leaves textInput with non-blank text but no
// resolved slug - a deliberately empty slot is never marked invalid.
function markUnresolvedOnBlur(textInput, slugInput) {
  textInput.classList.toggle("error", textInput.value.trim() !== "" && slugInput.value === "");
}

// selectOption fills textInput/slugInput from option, clears the invalid
// marker, and closes the suggestion list - the only place a hidden slug is
// ever set to a non-empty value.
function selectOption(slot, textInput, slugInput, option) {
  textInput.value = option.label;
  slugInput.value = option.id;
  textInput.classList.remove("error");
  closeSuggestions(slot);
  updateGroupCheck(slot.closest(".award-group"));
}

// renderSuggestions shows up to MAX_SUGGESTIONS matches in slot's own
// suggestion list, filtered by a case-insensitive substring match on label.
// An empty query renders no suggestions at all.
function renderSuggestions(slot, textInput, slugInput, position) {
  closeSuggestions(slot);

  const query = textInput.value.trim().toLowerCase();
  if (query === "") {
    return;
  }

  const matches = optionsForPosition(position)
    .filter((option) => option.label.toLowerCase().includes(query))
    .slice(0, MAX_SUGGESTIONS);
  if (matches.length === 0) {
    return;
  }

  const list = document.createElement("div");
  list.className = "finalist-suggestions";
  matches.forEach((option) => {
    const item = document.createElement("button");
    item.type = "button";
    item.className = "finalist-suggestion";
    item.textContent = option.label;
    // mousedown (not click) fires before the text input's own blur
    // handler, so the mouse path's selection wins over blur's "close on
    // focus loss." A keyboard-only user (Tab to the suggestion, then
    // Enter/Space) never fires mousedown - only a native click - so click
    // is also wired, calling the same idempotent selectOption.
    item.addEventListener("mousedown", (event) => {
      event.preventDefault();
      selectOption(slot, textInput, slugInput, option);
    });
    item.addEventListener("click", () => {
      selectOption(slot, textInput, slugInput, option);
    });
    list.appendChild(item);
  });
  slot.appendChild(list);
}

// initFinalistSlot wires one slot's text input: typing clears any stale
// slug and re-filters suggestions; Enter is prevented from submitting the
// whole 15-slot form mid-search; blurring closes the suggestion list and
// marks the field invalid if it's left unresolved, after a short delay so
// a suggestion's own mousedown still fires first.
function initFinalistSlot(slot) {
  const textInput = slot.querySelector(".finalist-text");
  const slugInput = slot.querySelector(".finalist-slug");
  const position = slot.dataset.position;
  if (!textInput || !slugInput || !position) {
    return;
  }

  textInput.addEventListener("input", () => {
    clearSlug(slugInput);
    renderSuggestions(slot, textInput, slugInput, position);
    updateGroupCheck(slot.closest(".award-group"));
  });

  textInput.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      event.preventDefault();
    }
  });

  textInput.addEventListener("blur", () => {
    window.setTimeout(() => {
      closeSuggestions(slot);
      markUnresolvedOnBlur(textInput, slugInput);
    }, 0);
  });
}

// A closed sheet's inputs already render server-disabled (sheet.html), and
// read-only means read-only: this script must not run at all there.
const awardsForm = document.getElementById("awards-form");
if (awardsForm && !awardsForm.hasAttribute("data-closed")) {
  awardsForm.querySelectorAll(".finalist-slot").forEach(initFinalistSlot);
}

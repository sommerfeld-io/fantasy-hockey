/*
  divisions.js drives the "divisions" pick sheet's live-cap dim/disable
  behavior (DESIGN.md's Division-picks progress indicator; AD-10): a
  division caps at DIVISION_CAP selected chips, a conference caps at
  CONFERENCE_CAP across its two divisions. This is the app's first
  JavaScript file, and it is cosmetic-immediacy only (AD-10's "the client
  disables/dims already-capped sibling checkboxes on change") - the server
  (handleDivisionsSubmit) independently re-validates and enforces every cap
  on submit regardless of what this script allows, so a disabled-JS or
  bypassing client still can't persist an invalid pick.
*/

const DIVISION_CAP = 5;
const CONFERENCE_CAP = 8;

// setChipCapped applies capped's disabled/dim state to input's own chip -
// the single place either cap (division or conference) writes to the DOM,
// so the two scopes can't overwrite each other's decision.
function setChipCapped(input, capped) {
  input.disabled = capped;
  const chip = input.closest(".chip");
  if (chip) {
    chip.classList.toggle("chip--capped", capped);
  }
}

// applyCaps recomputes every division's and conference's live count and
// text, then - for every chip - combines both scopes' cap decisions before
// touching the DOM once, so a division already at its own cap can't be
// re-enabled by the conference pass (or vice versa). Re-run on every chip
// checkbox change.
function applyCaps(form) {
  const divisionCapped = new Set();
  const conferenceCapped = new Set();

  form.querySelectorAll(".division-group").forEach((group) => {
    const inputs = Array.from(group.querySelectorAll(".chip-input"));
    const checkedCount = inputs.filter((input) => input.checked).length;

    const counter = group.querySelector('[data-role="division-count"]');
    if (counter) {
      counter.textContent = checkedCount + "/" + DIVISION_CAP;
    }
    inputs.forEach((input) => {
      if (!input.checked && checkedCount >= DIVISION_CAP) {
        divisionCapped.add(input);
      }
    });
  });

  form.querySelectorAll(".conference-group").forEach((conference) => {
    const inputs = Array.from(conference.querySelectorAll(".chip-input"));
    const checkedCount = inputs.filter((input) => input.checked).length;
    const valid = checkedCount === CONFERENCE_CAP;

    const indicator = conference.querySelector('[data-role="conference-count"]');
    if (indicator) {
      indicator.classList.toggle("count-indicator--valid", valid);
      indicator.classList.toggle("count-indicator--invalid", !valid);
      const label = indicator.querySelector("span");
      if (label) {
        label.textContent = checkedCount + "/" + CONFERENCE_CAP + " selected";
      }
    }
    inputs.forEach((input) => {
      if (!input.checked && checkedCount >= CONFERENCE_CAP) {
        conferenceCapped.add(input);
      }
    });
  });

  form.querySelectorAll(".chip-input").forEach((input) => {
    setChipCapped(input, divisionCapped.has(input) || conferenceCapped.has(input));
  });
}

// updateSubmitState keeps the submit button's disabled attribute in sync
// with every conference currently totaling exactly CONFERENCE_CAP -
// cosmetic-immediacy only; handleDivisionsSubmit still re-validates and
// rejects server-side regardless.
function updateSubmitState(form) {
  const submit = form.querySelector("#divisions-submit");
  if (!submit) {
    return;
  }

  const conferences = Array.from(form.querySelectorAll(".conference-group"));
  const allValid = conferences.every((conference) => {
    const inputs = conference.querySelectorAll(".chip-input");
    const checkedCount = Array.from(inputs).filter((input) => input.checked).length;
    return checkedCount === CONFERENCE_CAP;
  });
  submit.disabled = !allValid;
}

// initDivisionsForm wires the change listener and runs an initial pass so a
// reopened, pre-checked form starts with correct caps/counts/button state.
function initDivisionsForm(form) {
  applyCaps(form);
  updateSubmitState(form);

  form.addEventListener("change", (event) => {
    if (!event.target.classList || !event.target.classList.contains("chip-input")) {
      return;
    }
    applyCaps(form);
    updateSubmitState(form);
  });
}

// A closed sheet's chips/select already render server-disabled
// (sheet.html), and read-only means read-only: this script must not run at
// all there, since applyCaps only ever disables an unchecked chip past its
// cap - on a closed, already-checked chip it would otherwise flip disabled
// back to false on load.
const divisionsForm = document.getElementById("divisions-form");
if (divisionsForm && !divisionsForm.hasAttribute("data-closed")) {
  initDivisionsForm(divisionsForm);
}

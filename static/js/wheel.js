// wheel.js — Spin animation for Battle Bracket Wheels.
//
// Listens for the "spin-wheel" custom event fired by HTMX after
// a response includes the HX-Trigger header with a "spin-wheel" key.
// Handles both single-wheel spins (POST /wheel/{id}/spin) and
// two-wheel battle spins (POST /battle/{matchID}) where the detail
// is an array of spin data objects.
//
// Rotates the SVG wheel group to the target angle with a smooth
// CSS transition, adding multiple full rotations for a realistic
// spin effect.
//
// Results (match winner/loser, next-round slots, center display) are
// hidden during the spin animation via the pending-reveal CSS class
// and revealed after the animation completes.

(function () {
  "use strict";

  // Number of full 360-degree rotations before landing on the target.
  var FULL_SPINS = 5;

  // Duration of the spin animation in milliseconds (matches CSS transition).
  var SPIN_DURATION_MS = 3500;

  // Delay after animation before revealing results (slightly longer than
  // the CSS transition to ensure visual smoothness).
  var REVEAL_DELAY_MS = 3700;

  // Timer ID for the reveal timeout, to avoid multiple reveals.
  var revealTimer = null;

  function spinWheel(data) {
    var wheelID = data.wheelID;
    var slotID = data.slotID || "";
    var targetAngle = data.targetAngle;
    if (wheelID === undefined || targetAngle === undefined) return;

    // Find the wheel's rotating group element using scoped ID.
    var scopedGroupID = slotID
      ? "wheel-group-" + slotID + "-" + wheelID
      : "wheel-group-" + wheelID;
    var group = document.getElementById(scopedGroupID);
    if (!group) return;

    // Calculate total rotation: full spins + target angle.
    var totalAngle = FULL_SPINS * 360 + targetAngle;

    // Reset transition and transform to restart animation cleanly.
    // This handles the case where the wheel was already spun.
    group.style.transition = "none";
    group.style.transform = "rotate(0deg)";
    group.style.transformOrigin = "100px 100px";
    group.style.transformBox = "view-box";

    // Use requestAnimationFrame to separate the reset from the animated
    // rotation.  SVG <g> elements do NOT have offsetHeight (it returns
    // undefined), so the standard reflow-force trick does not work on
    // them.  Without a frame boundary, the browser batches all style
    // changes together and the CSS transition never starts.
    requestAnimationFrame(function () {
      group.style.transition =
        "transform 3.5s cubic-bezier(0.17, 0.67, 0.12, 0.99)";
      group.style.transform = "rotate(" + totalAngle + "deg)";
    });
  }

  // Reveal all pending results after the spin animation completes.
  function revealResults() {
    var pending = document.querySelectorAll(".pending-reveal");
    for (var i = 0; i < pending.length; i++) {
      pending[i].classList.remove("pending-reveal");
      pending[i].classList.add("revealed");
    }
    revealTimer = null;
  }

  // Schedule reveal after the spin animation finishes.
  function scheduleReveal() {
    // Clear any existing timer so we don't stack reveals.
    if (revealTimer !== null) {
      clearTimeout(revealTimer);
    }
    // Wait for animation to complete, then reveal results.
    revealTimer = setTimeout(revealResults, REVEAL_DELAY_MS);
  }

  document.addEventListener("spin-wheel", function (evt) {
    // HTMX 2.x wraps the trigger value in {elt: ..., value: ...}.
    // HTMX 1.x and manual dispatches set detail directly to the value.
    var detail = evt.detail;
    if (!detail) return;

    var triggerValue =
      detail.value !== undefined ? detail.value : detail;

    // The battle handler sends an array of spin data objects
    // (one per wheel), while the single-spin handler sends a
    // single object.  Normalise to an array for uniform handling.
    var items = Array.isArray(triggerValue) ? triggerValue : [triggerValue];

    for (var i = 0; i < items.length; i++) {
      spinWheel(items[i]);
    }

    // Schedule result reveal after animation completes.
    // Only schedule if this is a battle (2 items — two wheels spinning).
    // Single-wheel spins (1 item) are not battles and have no results to reveal.
    if (items.length > 1) {
      scheduleReveal();
    }
  });
})();

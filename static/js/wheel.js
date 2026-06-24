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

(function () {
  "use strict";

  // Number of full 360-degree rotations before landing on the target.
  var FULL_SPINS = 5;

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

    // Force a synchronous reflow so the reset takes effect before
    // we apply the animated rotation.
    group.offsetHeight; // eslint-disable-line no-unused-expressions

    // Apply the spin with a deceleration (ease-out) curve.
    group.style.transition =
      "transform 3.5s cubic-bezier(0.17, 0.67, 0.12, 0.99)";
    group.style.transformOrigin = "100px 100px";
    group.style.transform = "rotate(" + totalAngle + "deg)";
  }

  document.addEventListener("spin-wheel", function (evt) {
    var detail = evt.detail;
    if (!detail) return;

    // The battle handler sends an array of spin data objects
    // (one per wheel), while the single-spin handler sends a
    // single object.  Normalise to an array for uniform handling.
    var items = Array.isArray(detail) ? detail : [detail];

    for (var i = 0; i < items.length; i++) {
      spinWheel(items[i]);
    }
  });
})();

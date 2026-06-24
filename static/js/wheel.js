// wheel.js — Spin animation for Battle Bracket Wheels.
//
// Listens for the "spin-wheel" custom event fired by HTMX after
// a POST /wheel/{id}/spin response includes the HX-Trigger header.
// Rotates the SVG wheel group to the target angle with a smooth
// CSS transition, adding multiple full rotations for a realistic
// spin effect.

(function () {
  "use strict";

  // Number of full 360-degree rotations before landing on the target.
  var FULL_SPINS = 5;

  document.addEventListener("spin-wheel", function (evt) {
    var detail = evt.detail;
    if (!detail) return;

    var wheelID = detail.wheelID;
    var slotID = detail.slotID || "";
    var targetAngle = detail.targetAngle;
    if (wheelID === undefined || targetAngle === undefined) return;

    // Find the wheel's rotating group element using scoped ID.
    var scopedGroupID = slotID ? "wheel-group-" + slotID + "-" + wheelID : "wheel-group-" + wheelID;
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
  });
})();

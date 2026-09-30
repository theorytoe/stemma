// Theme toggle.
//
// This is polish, not function: with JavaScript off the button stays hidden and
// the stylesheet's prefers-color-scheme rules decide the theme, so the site
// reads exactly the same. The script only adds an explicit choice on top, and
// remembers it.
(function () {
  var root = document.documentElement;
  var KEY = "stemma-theme";

  function stored() {
    try {
      var v = localStorage.getItem(KEY);
      return v === "light" || v === "dark" ? v : null;
    } catch (e) {
      return null;
    }
  }

  function prefer() {
    return window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";
  }

  // Applied before first paint, so a remembered choice does not flash the other
  // theme first.
  var choice = stored();
  if (choice) {
    root.setAttribute("data-theme", choice);
  }

  function toggle() {
    var next = (root.getAttribute("data-theme") || prefer()) === "dark"
      ? "light"
      : "dark";
    root.setAttribute("data-theme", next);
    try {
      localStorage.setItem(KEY, next);
    } catch (e) {}
  }

  function show() {
    var button = document.querySelector("[data-theme-toggle]");
    if (!button) {
      return;
    }
    button.hidden = false;
    button.addEventListener("click", toggle);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", show);
  } else {
    show();
  }
})();

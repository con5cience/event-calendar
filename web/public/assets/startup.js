// Runs synchronously before body parsing. No inline script or CSP exception.
(() => {
  const root = document.documentElement;
  root.classList.add("app-booting");
  function finish() {
    root.classList.remove("app-booting");
    clearTimeout(timeout);
    window.removeEventListener("error", onError, true);
    window.removeEventListener("calendar-startup-ready", finish);
    window.removeEventListener("calendar-startup-failed", finish);
  }
  function onError(event) {
    if (
      event instanceof ErrorEvent ||
      event.target instanceof HTMLScriptElement
    )
      finish();
  }
  // A stalled bundle/configuration request must not leave the page hidden forever.
  const timeout = setTimeout(finish, 15000);
  window.addEventListener("error", onError, true);
  window.addEventListener("calendar-startup-ready", finish);
  window.addEventListener("calendar-startup-failed", finish);
})();

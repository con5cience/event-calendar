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
    const entries = [
      ...document.querySelectorAll('script[type="module"][src]'),
    ].filter((script) => new URL(script.src).origin === location.origin);
    // Optional injected scripts (including blocked analytics) do not determine
    // whether the calendar can start. The timeout still covers unknown failures.
    if (
      entries.some(
        (entry) =>
          event.target === entry ||
          (event instanceof ErrorEvent && event.filename === entry.src),
      )
    )
      finish();
  }
  // A stalled bundle/configuration request must not leave the page hidden forever.
  const timeout = setTimeout(finish, 15000);
  window.addEventListener("error", onError, true);
  window.addEventListener("calendar-startup-ready", finish);
  window.addEventListener("calendar-startup-failed", finish);
})();

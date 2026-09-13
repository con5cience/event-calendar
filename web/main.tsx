import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import "@fullcalendar/react/skeleton.css";
import "@fullcalendar/react/themes/monarch/theme.css";
import "@fullcalendar/react/themes/monarch/palettes/green.css";
import "./style.css";
import { configureSite } from "./site";

fetch("/api/site")
  .then((response) => {
    if (!response.ok) throw new Error("Site configuration is unavailable");
    return response.json();
  })
  .then((value) => {
    configureSite(value);
    createRoot(document.getElementById("root")!).render(
      <StrictMode>
        <App />
      </StrictMode>,
    );
  })
  .catch(() => {
    window.dispatchEvent(new Event("calendar-startup-failed"));
    const message = document.createElement("p");
    message.setAttribute("role", "alert");
    message.textContent =
      "Unable to load site configuration. Please try again later.";
    document.getElementById("root")!.append(message);
  });

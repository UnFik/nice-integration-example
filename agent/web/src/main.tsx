import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { Session } from "./session";
import "./styles.css";

const session = new Session({
  fetch: window.fetch.bind(window),
  navigate: (url) => window.location.assign(url),
  replaceUrl: (path) => window.history.replaceState(null, "", path),
});

createRoot(document.getElementById("root") as HTMLElement).render(
  <StrictMode>
    <App session={session} />
  </StrictMode>,
);

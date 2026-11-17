import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { readConfig } from "./config";
import { Session } from "./session";
import { ConfigErrorPage } from "./ui/ConfigErrorPage";
import "./styles.css";

const root = createRoot(document.getElementById("root") as HTMLElement);
const result = readConfig(import.meta.env, window.location.origin);

if (result.ok) {
  const session = new Session(result.config, {
    storage: window.sessionStorage,
    fetch: window.fetch.bind(window),
    navigate: (url) => window.location.assign(url),
    replaceUrl: (path) => window.history.replaceState(null, "", path),
  });
  root.render(
    <StrictMode>
      <App session={session} />
    </StrictMode>,
  );
} else {
  root.render(
    <StrictMode>
      <ConfigErrorPage errors={result.errors} />
    </StrictMode>,
  );
}

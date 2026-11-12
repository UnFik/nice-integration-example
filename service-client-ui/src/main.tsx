import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { readConfig } from "./config";
import { ConfigErrorPage } from "./ui/ConfigErrorPage";
import "./styles.css";

const root = createRoot(document.getElementById("root") as HTMLElement);
const result = readConfig(import.meta.env, window.location.origin);

root.render(<StrictMode>{result.ok ? <p>NICE: {result.config.issuer}</p> : <ConfigErrorPage errors={result.errors} />}</StrictMode>);

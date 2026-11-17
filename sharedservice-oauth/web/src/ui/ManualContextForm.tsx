import { type FormEvent, useId, useState } from "react";
import type { ActiveContext } from "../contexts";
import { type ManualContextError, validateManualContext } from "../recent-contexts";
import { shortId } from "./format";

export function ManualContextForm(props: {
  defaultOrg: string | undefined;
  recent: ActiveContext[];
  busy: boolean;
  onUse: (org: string, project: string) => Promise<boolean>;
}) {
  const { busy, onUse } = props;
  const id = useId();
  const [org, setOrg] = useState<string>();
  const [project, setProject] = useState("");
  const [error, setError] = useState<ManualContextError>();
  const orgValue = org ?? props.defaultOrg ?? "";

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const checked = validateManualContext(orgValue, project);
    if ("field" in checked) {
      setError(checked);
      return;
    }
    setError(undefined);
    await onUse(checked.org, checked.project);
  };

  return (
    <div className="manual">
      <form onSubmit={(event) => void submit(event)} noValidate>
        <h3>Pilih konteks manual</h3>
        <div className="field">
          <label htmlFor={`${id}-org`}>ID organisasi</label>
          <input
            id={`${id}-org`}
            name="org"
            value={orgValue}
            onChange={(event) => setOrg(event.target.value)}
            autoComplete="off"
            spellCheck={false}
            placeholder="org_…"
            aria-invalid={error?.field === "org" || undefined}
            aria-describedby={error?.field === "org" ? `${id}-org-error` : undefined}
          />
          {error?.field === "org" && (
            <p id={`${id}-org-error`} className="field-error">
              {error.message}
            </p>
          )}
        </div>
        <div className="field">
          <label htmlFor={`${id}-project`}>ID project</label>
          <input
            id={`${id}-project`}
            name="project"
            value={project}
            onChange={(event) => setProject(event.target.value)}
            autoComplete="off"
            spellCheck={false}
            placeholder="prj_…"
            aria-invalid={error?.field === "project" || undefined}
            aria-describedby={error?.field === "project" ? `${id}-project-error` : `${id}-project-hint`}
          />
          {error?.field === "project" ? (
            <p id={`${id}-project-error`} className="field-error">
              {error.message}
            </p>
          ) : (
            <p id={`${id}-project-hint`} className="hint">
              Organisasi sesi sudah terisi. Ganti bila project ada di organisasi lain.
            </p>
          )}
        </div>
        <button type="submit" className="primary block" disabled={busy}>
          Gunakan konteks ini
        </button>
      </form>
      {props.recent.length > 0 && (
        <div className="recent">
          <h3>Terakhir dipakai</h3>
          <ul className="choices">
            {props.recent.map((entry) => (
              <li key={`${entry.org}/${entry.project}`}>
                <button
                  type="button"
                  className="choice"
                  data-recent={`${entry.org}/${entry.project}`}
                  disabled={busy}
                  onClick={() => entry.project && void onUse(entry.org, entry.project)}
                >
                  <span className="choice-name">
                    <code title={entry.org}>{shortId(entry.org)}</code> / <code title={entry.project}>{shortId(entry.project)}</code>
                  </span>
                </button>
              </li>
            ))}
          </ul>
          <p className="hint">Hanya ID, diingat selama tab ini terbuka.</p>
        </div>
      )}
    </div>
  );
}

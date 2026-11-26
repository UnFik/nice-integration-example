import { useId } from "react";
import type { Session, SessionState } from "../session";
import { shortId } from "./format";

const NONE = "";

// ContextSelect offers only what GET /auth/contexts answered: it never guesses
// an organization or project. Choosing asks the backend for one refresh.
export function ContextSelect({ session, state }: { session: Session; state: SessionState }) {
  const id = useId();
  const active = state.active;
  const value = active ? `${active.org}/${active.project}` : NONE;
  const contexts = state.contexts;
  const known = contexts?.organizations.some((org) => org.id === active?.org && org.projects.some((project) => project.id === active?.project));
  const busy = state.switching !== undefined || state.turn !== undefined || !contexts;

  return (
    <div className="context-select">
      <label htmlFor={id} className="muted small">
        Konteks
      </label>
      <select
        id={id}
        value={value}
        disabled={busy}
        aria-busy={state.switching !== undefined || undefined}
        onChange={(event) => {
          const [org, project] = event.target.value.split("/");
          if (org && project) void session.choose(org, project);
        }}
      >
        {!active && <option value={NONE}>{contexts ? "Pilih project…" : "Memuat konteks…"}</option>}
        {active && !known && <option value={value}>{`${shortId(active.org)} / ${shortId(active.project)}`}</option>}
        {contexts?.organizations.map((org) => (
          <optgroup key={org.id} label={`${org.name || org.slug || org.id}${org.actAs ? " (operator platform)" : ""}`}>
            {org.projects.length === 0 ? (
              <option disabled value={`unavailable:${org.id}`}>
                Tidak ada project dengan akses agen
              </option>
            ) : (
              org.projects.map((project) => (
                <option key={project.id} value={`${org.id}/${project.id}`} data-context={`${org.id}/${project.id}`}>
                  {project.name || project.slug || project.id}
                </option>
              ))
            )}
          </optgroup>
        ))}
      </select>
    </div>
  );
}

import type { ActiveContext, Contexts } from "../contexts";

// ContextPicker offers only what /me/contexts answered: there is no free input,
// so the UI never guesses an organization or project.
export function ContextPicker(props: {
  contexts: Contexts;
  active: ActiveContext;
  busy: boolean;
  onChoose: (org: string, project: string) => void;
}) {
  const { contexts, active, busy, onChoose } = props;
  return (
    <div className="contexts">
      {contexts.organizations.map((org) => {
        const isCurrentOrg = org.id === active.org;
        return (
          <article key={org.id} className={`card${isCurrentOrg ? " current" : ""}`}>
            <header className="card-head">
              <h3>{org.name}</h3>
              {isCurrentOrg && !active.project && <span className="badge">Organisasi saat ini</span>}
            </header>
            <p className="muted">
              <code>{org.id}</code>
            </p>
            {org.projects.length === 0 ? (
              <p className="muted">
                Tidak tersedia: Anda tidak memegang akses inventaris di project mana pun di organisasi ini.
              </p>
            ) : (
              <ul className="choices">
                {org.projects.map((project) => {
                  const isActive = isCurrentOrg && project.id === active.project;
                  return (
                    <li key={project.id}>
                      <button
                        type="button"
                        className={isActive ? "" : "secondary"}
                        data-context={`${org.id}/${project.id}`}
                        aria-current={isActive ? "true" : undefined}
                        disabled={busy}
                        onClick={() => onChoose(org.id, project.id)}
                      >
                        {project.name}
                        {isActive && " (aktif)"}
                      </button>
                      <span className="muted"> service: {project.services.join(", ")}</span>
                    </li>
                  );
                })}
              </ul>
            )}
            {org.projectsTruncated && (
              <p className="muted">NICE hanya mengirim 100 project pertama dari organisasi ini.</p>
            )}
          </article>
        );
      })}
    </div>
  );
}

import type { ActiveContext, Contexts } from "../contexts";

// ContextPicker offers only what /me/contexts answered: it never guesses an
// organization or project. Typed ids live in ManualContextForm.
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
          <article key={org.id} className={`org${isCurrentOrg ? " current" : ""}`}>
            <header className="org-head">
              <h3>{org.name}</h3>
              {isCurrentOrg && !active.project && <span className="badge info">Organisasi saat ini</span>}
            </header>
            <p className="muted small">
              <code>{org.id}</code>
            </p>
            {org.projects.length === 0 ? (
              <p className="muted small">
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
                        className={`choice${isActive ? " active" : ""}`}
                        data-context={`${org.id}/${project.id}`}
                        aria-current={isActive ? "true" : undefined}
                        disabled={busy}
                        onClick={() => onChoose(org.id, project.id)}
                      >
                        <span className="choice-name">
                          {project.name}
                          {isActive && " (aktif)"}
                        </span>
                        <span className="muted small">service: {project.services.join(", ")}</span>
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}
            {org.projectsTruncated && (
              <p className="muted small">NICE hanya mengirim 100 project pertama dari organisasi ini.</p>
            )}
          </article>
        );
      })}
    </div>
  );
}

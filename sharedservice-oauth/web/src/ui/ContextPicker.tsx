import type { ActiveContext, Contexts } from "../api";

// ContextPicker offers only what GET /auth/contexts answered: it never guesses
// an organization or project.
export function ContextPicker(props: {
  contexts: Contexts;
  active?: ActiveContext;
  busy: boolean;
  onChoose: (org: string, project: string) => void;
}) {
  const { contexts, active, busy, onChoose } = props;
  // Before a context is chosen, the organization NICE picked at login stands for the current one.
  const currentOrg = active?.org ?? contexts.currentOrganizationId;
  return (
    <div className="contexts">
      {contexts.organizations.map((org) => {
        const isCurrentOrg = org.id === currentOrg;
        return (
          <article key={org.id} className={`org${isCurrentOrg ? " current" : ""}`}>
            <header className="org-head">
              <h3>{org.name || org.slug || org.id}</h3>
              {isCurrentOrg && !active && <span className="badge info">Organisasi saat ini</span>}
              {org.actAs && (
                <span className="badge" data-act-as={org.id} title="Anda bukan anggota organisasi ini; Anda masuk sebagai operator platform nadi.">
                  Operator platform
                </span>
              )}
            </header>
            <p className="muted small">
              <code>{org.id}</code>
            </p>
            {org.projects.length === 0 ? (
              <p className="muted small" data-unavailable={org.id}>
                Tidak tersedia: Anda tidak punya akses nadi di project mana pun di organisasi ini.
              </p>
            ) : (
              <ul className="choices">
                {org.projects.map((project) => {
                  const isActive = active?.org === org.id && active.project === project.id;
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
                          {project.name || project.slug || project.id}
                          {isActive && " (aktif)"}
                        </span>
                        <span className="muted small">{project.slug || project.id}</span>
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}
            {org.projectsTruncated && <p className="muted small">NICE tidak mengirim semua project dari organisasi ini.</p>}
          </article>
        );
      })}
    </div>
  );
}

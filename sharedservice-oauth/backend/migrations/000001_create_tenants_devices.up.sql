-- IDs of NICE (organization, project, user, client) are kept as text without a
-- foreign key: NICE owns them, and names are read from NICE when shown.
CREATE TABLE tenants (
    org_id     text PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE devices (
    id          uuid PRIMARY KEY,
    org_id      text NOT NULL REFERENCES tenants (org_id),
    project_id  text NOT NULL,
    created_by  text NOT NULL,
    acted_by    text,
    name        text NOT NULL,
    serial_number text NOT NULL,
    status      text NOT NULL CHECK (status IN ('active', 'inactive', 'maintenance')),
    location    text NOT NULL DEFAULT '',
    create_time timestamptz NOT NULL DEFAULT now(),
    update_time timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX devices_scope_idx ON devices (org_id, project_id);

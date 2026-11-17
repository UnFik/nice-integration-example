-- IDs of NICE (organization, project, user, client) are kept as text without a
-- foreign key: NICE owns them, and names are read from NICE when shown.
CREATE TABLE tenants (
    org_id     text PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE items (
    id          uuid PRIMARY KEY,
    org_id      text NOT NULL REFERENCES tenants (org_id),
    project_id  text NOT NULL,
    created_by  text NOT NULL,
    acted_by    text,
    name        text NOT NULL,
    quantity    bigint NOT NULL CHECK (quantity >= 0),
    create_time timestamptz NOT NULL DEFAULT now(),
    update_time timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX items_scope_idx ON items (org_id, project_id);

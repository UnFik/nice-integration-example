## ADDED Requirements

### Requirement: Penyelesaian `perms_ref` tanpa kredensial tambahan
The agent SHALL always configure the `perms_ref` resolver of `verify` with the NICE issuer only, forwarding the token of the MCP request to NICE, and SHALL NOT read a NICE API key for it. `.env.example` and the README SHALL NOT mention `NICE_SERVICE_API_KEY`.

#### Scenario: Resolver tanpa konfigurasi
- **WHEN** the verifier config is built from a config that holds only the required variables
- **THEN** it carries a `*verify.HTTPResolver`

#### Scenario: Variabel lama tidak ada
- **WHEN** the configuration test reads `.env.example`
- **THEN** the file does not list `NICE_SERVICE_API_KEY`, and the README documents exactly the variables of `.env.example`

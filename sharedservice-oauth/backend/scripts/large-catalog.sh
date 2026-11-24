#!/usr/bin/env bash
# Prints the OpenAPI document of nadi with extra operations, to try what NICE does for a caller whose permissions do
# not fit a token: it issues the token with perms_ref in place of perms, and nadi resolves the list at NICE once.
# Import the output as the catalog of nadi, give a user the role nadi.admin, and sign in (README, "Menguji izin yang
# tidak muat di token").
#
#   scripts/large-catalog.sh [COUNT] > /tmp/nadi-large.json     COUNT defaults to 150
#
# Each extra operation (GET /catalog-test/pNNN) declares a permission of its own, nadi.catalog_test.pNNN.read. nadi
# does not serve them: they only make the permission list long. Import docs/swagger.json again to remove them.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

count=${1:-150}
if ! [[ $count =~ ^[0-9]+$ ]] || (( count < 1 || count > 500 )); then
  printf 'GAGAL: COUNT harus bilangan 1 sampai 500, bukan %s\n' "$count" >&2
  exit 1
fi
command -v jq >/dev/null || { printf "GAGAL: perintah 'jq' tidak ditemukan; pasang dulu.\n" >&2; exit 1; }

jq --argjson n "$count" '
  .paths += ([range(0; $n)] | map(("00" + tostring)[-3:] as $id | {
    key: ("/catalog-test/p" + $id),
    value: {get: {
      security: [{BearerAuth: []}],
      tags: ["catalog-test"],
      summary: ("Catalog test operation " + $id),
      description: "Not served by nadi. It only adds a permission to the catalog, to make the permission list of a caller longer than a token carries.",
      operationId: ("catalogTest" + $id),
      responses: {"404": {description: "Not Found"}},
      "x-nice-permission": ("nadi.catalog_test.p" + $id + ".read")
    }}
  }) | from_entries)
' docs/swagger.json

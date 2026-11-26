# Spec Delta

## MODIFIED Requirements

### Requirement: Discovery service dan operasi dari NICE
The agent SHALL offer the tools `list_services` and `list_operations`, answered from NICE's caller service map and service operations (via the `credentials` package of `github.com/ABA-Developer/nice-auth-sdk` at `v0.2.5` or later), with the MCP request's token as subject token. It SHALL NOT keep a hand-written list of services, operations, or service addresses. Each operation of `list_operations` SHALL carry the description NICE gives, and each request field its description, format, allowed values, and the mark that the allowed values were cut, leaving out what NICE does not send.

#### Scenario: Peta service
- **WHEN** a user who holds `nadi` permissions in project Demo calls `list_services`
- **THEN** the result lists the user's organizations, each with its projects and the slugs of the services callable in each (including `nadi` under Demo), and the name and description of every listed service

#### Scenario: Proyek terpotong
- **WHEN** NICE marks an organization of the map as `projects_truncated`
- **THEN** the result of `list_services` marks that organization as incomplete

#### Scenario: Operasi satu service
- **WHEN** the user calls `list_operations` with `service` `nadi`
- **THEN** the result lists the operations of `nadi` whose permission the user holds, each with its key, method, path, summary, description, permission, request fields, and whether it is sensitive

#### Scenario: Field dengan pilihan nilai
- **WHEN** NICE lists the body field `status` of `createDevice` with `enum` `["active","inactive","maintenance"]`
- **THEN** that field in the result of `list_operations` carries the same allowed values

#### Scenario: Service yang tidak boleh dipanggil
- **WHEN** the user calls `list_operations` with a slug NICE answers `404 not_found` for
- **THEN** the tool result is an error saying the user may not call that service, and no service is called

## ADDED Requirements

### Requirement: Daftar operasi diperiksa ulang dengan ETag
The agent SHALL keep the operations of a service per user and service in memory, bounded in the number of entries, with the `ETag` NICE gave. Every read of the operations SHALL ask NICE with that `ETag` as `If-None-Match`, and SHALL use the kept list only when NICE answers 304. A kept list SHALL NOT be used without asking NICE.

#### Scenario: Daftar tidak berubah
- **WHEN** the same user calls `list_operations` for `nadi` twice and nothing changed in NICE
- **THEN** the second call asks NICE with the `ETag` of the first, NICE answers 304, and the result is the same list

#### Scenario: Izin dicabut
- **WHEN** a permission of the user is removed between two calls of `list_operations`
- **THEN** the second call returns the list without the operations of that permission

### Requirement: Skema operasi untuk model
The agent SHALL offer the read-only tool `get_operation_schema` taking `service` and `operations`, 1 to 10 keys from `list_operations`. It SHALL answer, for each key the user may call, the input schema and the response schemas as NICE gives them and whether NICE cut them, and the keys the user may not call or that are not active as missing.

#### Scenario: Skema body bersarang
- **WHEN** the user calls `get_operation_schema` with `service` `nadi` and `operations` `["createDevice"]`
- **THEN** the result holds the input schema of `createDevice` with its body properties and required names, and its response schemas by status

#### Scenario: Key yang tidak boleh dipanggil
- **WHEN** the user who may not delete devices asks for `listDevices` and `deleteDevice`
- **THEN** the result holds the schema of `listDevices` and names `deleteDevice` missing

#### Scenario: Terlalu banyak key
- **WHEN** `operations` holds no key or more than 10 keys
- **THEN** the tool result is an `invalid_argument` error and NICE is not called

#### Scenario: NICE tanpa skema
- **WHEN** NICE answers 404 for the schemas because it does not serve them yet
- **THEN** the tool result is an error saying NICE does not serve operation schemas

### Requirement: Field rahasia tidak sampai ke model
Before `call_operation` sends a call, the agent SHALL read the schema of that operation from NICE. It SHALL remove from the answer of the service every property that the response schema of its status marks `x-secret: true`, before the answer reaches the model, the conversation history, or a tool error. When the schema cannot be read for a temporary reason, the call SHALL NOT be sent. When NICE does not serve schemas (404), the call SHALL proceed and the answer SHALL be returned as it is.

#### Scenario: Properti rahasia dibuang
- **WHEN** the document of `nadi` marks `acted_by` of its device list answer `x-secret: true` and the user lists devices through `call_operation`
- **THEN** the result of the tool holds the answer without `acted_by`, and every other property unchanged

#### Scenario: Skema tidak terbaca
- **WHEN** NICE answers 503 when the agent reads the schema of the operation
- **THEN** the tool result is a temporary error and the service is not called

#### Scenario: NICE tanpa skema
- **WHEN** NICE answers 404 for the schema of the operation
- **THEN** the call is sent and its answer is returned as it is

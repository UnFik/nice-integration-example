# Spec Delta

## ADDED Requirements

### Requirement: Aktor dicatat pada barang
When the verified token carries `act`, the service SHALL record `act.sub` as `acted_by` on items it creates and SHALL return `acted_by` with the item; without `act`, `acted_by` SHALL be absent. The user (`sub`) SHALL remain the creator.

#### Scenario: Barang dibuat agen
- **WHEN** an item is created with a token whose `sub` is user U and `act.sub` is the client of `agen`
- **THEN** the item has `created_by` U and `acted_by` the client of `agen`

#### Scenario: Barang dibuat user langsung
- **WHEN** an item is created with a token without `act`
- **THEN** the item has no `acted_by`

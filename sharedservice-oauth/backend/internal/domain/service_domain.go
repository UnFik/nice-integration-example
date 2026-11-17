// Package domain holds the entities and business rules of the inventory service.
package domain

// ServiceSlug is the slug registered in the NICE catalog. Tokens for this
// service carry it as aud, and every permission name starts with it.
const ServiceSlug = "inventaris"

const (
	PermRead   = ServiceSlug + ".items.read"
	PermCreate = ServiceSlug + ".items.create"
	PermEdit   = ServiceSlug + ".items.edit"
	PermDelete = ServiceSlug + ".items.delete"
)

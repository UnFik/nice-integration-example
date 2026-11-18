// Package domain holds the entities and business rules of the NADI device service.
package domain

// ServiceSlug is the slug registered in the NICE catalog. Tokens for this
// service carry it as aud, and every permission name starts with it.
const ServiceSlug = "nadi"

const (
	PermRead   = ServiceSlug + ".devices.read"
	PermCreate = ServiceSlug + ".devices.create"
	PermEdit   = ServiceSlug + ".devices.edit"
	PermDelete = ServiceSlug + ".devices.delete"
)

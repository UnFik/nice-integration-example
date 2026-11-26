package calldto

// ServiceMap is where the user may call which service, as NICE lists it. It holds the endpoint of each service, so
// it stays inside the agent; the model reads ServicesView.
type ServiceMap struct {
	Services      []MapService
	Organizations []MapOrganization
}

// MapService is a service of the map as the catalog of NICE describes it.
type MapService struct {
	Slug        string
	Name        string
	Description string
	Endpoint    string
	BasePath    string
}

// MapOrganization is an organization of the user with the projects where a service may be called.
type MapOrganization struct {
	ID                string
	Name              string
	Slug              string
	Projects          []MapProject
	ProjectsTruncated bool
}

// MapProject is a project with the slugs of the services the user may call in it.
type MapProject struct {
	ID       string
	Name     string
	Slug     string
	Services []string
}

// ServicesView is the answer of list_services: the map without the addresses of the services.
type ServicesView struct {
	Services      []ServiceView      `json:"services"`
	Organizations []OrganizationView `json:"organizations"`
}

type ServiceView struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type OrganizationView struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Projects []ProjectView `json:"projects"`
	// Incomplete tells the model that NICE listed only part of the projects of the organization.
	Incomplete bool `json:"projects_truncated,omitempty"`
}

type ProjectView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Services []string `json:"services"`
}

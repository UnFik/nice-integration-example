package calldto

// Request is one call to a service. A nil Body sends no body.
type Request struct {
	Method string
	URL    string
	Body   []byte
}

// Response is the answer of a service, with the body cut at a size limit.
type Response struct {
	Status int
	Body   []byte
}

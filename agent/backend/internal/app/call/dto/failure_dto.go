package calldto

// Failure is a failed call as the model and the user read it.
type Failure struct {
	Code        string
	Description string
	Message     string
}

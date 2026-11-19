package devicedto

// Failure is a failed call as the operator reads it. It never holds the API key, a token, or a raw answer.
type Failure struct {
	// Code and Description are those NICE or nadi gave, or a code of this client when neither answered.
	Code        string
	Description string
	// Message says what the code means here.
	Message string
	// Step is the step of the README setup checklist to look at; empty when none applies.
	Step string
	// Temporary is true when trying again later may work without anyone changing the setup.
	Temporary bool
}

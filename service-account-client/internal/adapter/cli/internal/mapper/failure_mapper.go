package mapper

import (
	"fmt"
	"io"

	devicedto "github.com/nice-platform/example/service-account-client/internal/app/device/dto"
)

// WriteFailure prints a failure: the code, the description NICE or nadi gave, what it means here, and the step of
// the README setup checklist to look at.
func WriteFailure(w io.Writer, f *devicedto.Failure) {
	head := f.Code
	if f.Description != "" && f.Description != f.Code {
		head += " (" + f.Description + ")"
	}
	_, _ = fmt.Fprintf(w, "FAILED: %s: %s\n", head, f.Message)
	if f.Step != "" {
		_, _ = fmt.Fprintf(w, "        see README, setup checklist step %s\n", f.Step)
	}
}

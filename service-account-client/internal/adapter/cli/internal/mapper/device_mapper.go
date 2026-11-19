// Package mapper writes devices and failures for the terminal.
package mapper

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/nice-platform/example/service-account-client/internal/domain"
)

// WriteDevices prints the devices as an aligned table.
func WriteDevices(w io.Writer, devices []domain.Device) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tSERIAL\tSTATUS\tLOCATION\tCREATED BY")
	for _, d := range devices {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", d.ID, d.Name, d.SerialNumber, d.Status, d.Location, d.CreatedBy)
	}
	_ = tw.Flush()
}

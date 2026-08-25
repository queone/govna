// Package usererr formats plain, Govna-branded error text for Director-readable output.
package usererr

import "fmt"

// Errorf formats a user-facing error the same way every govna command surfaces one.
func Errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

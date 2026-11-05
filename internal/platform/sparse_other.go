//go:build !windows

package platform

import "os"

// setSparse is a no-op: extending a file with Truncate is already sparse here.
func setSparse(*os.File) {}

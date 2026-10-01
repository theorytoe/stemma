//go:build !unix

package cli

import "os"

// widthOf has no portable meaning away from a Unix terminal, so the width is
// unknown and the caller prints without clipping.
func widthOf(*os.File) int { return 0 }

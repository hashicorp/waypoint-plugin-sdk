package docs

import "strings"

var typeCleanupPatterns = strings.NewReplacer(
	"*", "",
	"[]", "list of ",
	"map[", "map of ",
	"]", " to ",
)

// Cleanup the type to make it nicer to read in docs.
func cleanupType(t string) string {
	return typeCleanupPatterns.Replace(t)
}

// Package version holds the version string the tool reports about itself.
//
// It exists so the version has one home: the user agent resolution sends, and
// anything else that needs to name the build, all read it here. A build that
// does not stamp it reports the development default.
package version

// Version is the release version. A build may replace it:
//
//	go build -ldflags "-X github.com/theorytoe/stemma/internal/version.Version=v1.2.3"
var Version = "0.0.0-dev"

// UserAgent is what the tool calls itself when it makes a request: the version,
// and where to find out what asked.
//
// It lives here rather than at either call site because two independent parts of
// the tool make requests — resolution and extraction — and they have to give the
// same answer to "who are you?". A second spelling would be a second user agent
// to keep true.
func UserAgent() string {
	return "stemma/" + Version + " (+https://github.com/theorytoe/stemma)"
}

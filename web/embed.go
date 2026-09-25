// Package web embeds the static phone hazard-report page. It exists so
// internal/httpapi can serve web/phone.html via go:embed without needing a
// relative pattern that escapes its own directory (go:embed patterns can't
// contain "..").
package web

import _ "embed"

//go:embed phone.html
var PhoneHTML []byte

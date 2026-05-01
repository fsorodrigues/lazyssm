package process

import "strings"

const unsupportedAuthPTYMessage = "auth PTY mode is unsupported on this platform"

func IsAuthPTYUnsupported(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), unsupportedAuthPTYMessage)
}

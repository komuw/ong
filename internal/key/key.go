// Package key implements some common secure functionality.
package key

import (
	"fmt"
	"unicode"
)

// IsSecure checks that secretKey has at least some minimum desirable security properties.
func IsSecure(secretKey string) error {
	const (
		minLen   = 6
		maxLen   = 256
		expected = 1
	)

	if len(secretKey) < minLen {
		return fmt.Errorf("ong: secretKey size is less than minimum required of %d", minLen)
	}
	if len(secretKey) > maxLen {
		return fmt.Errorf("ong: secretKey size is more than maximum required of %d", maxLen)
	}

	hasDigit := 0
	hasSymbol := 0
	hasLetter := 0
	for _, r := range secretKey {
		if unicode.IsDigit(r) {
			hasDigit = hasDigit + 1
		}
		if unicode.IsPunct(r) {
			hasSymbol = hasSymbol + 1
		}
		if unicode.IsLetter(r) {
			hasLetter = hasLetter + 1
		}
	}

	if hasDigit < expected {
		return fmt.Errorf("ong: secretKey should have at least %d digits", expected)
	}
	if hasSymbol < expected {
		return fmt.Errorf("ong: secretKey should have at least %d symbols", expected)
	}
	if hasLetter < expected {
		return fmt.Errorf("ong: secretKey should have at least %d letters", expected)
	}

	return nil
}

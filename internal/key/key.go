package key

import (
	"fmt"
	"unicode"
)

// IsSecure checks that k has a minimum of desirable security properties.
func IsSecure(k string) error {
	const (
		minLen   = 6
		maxLen   = 256
		expected = 1
	)

	if len(k) < minLen {
		return fmt.Errorf("ong/middleware: secretKey size is less than minimum required of %d", minLen)
	}
	if len(k) > maxLen {
		return fmt.Errorf("ong/middleware: secretKey size is more than maximum required of %d", maxLen)
	}

	hasDigit := 0
	hasSymbol := 0
	hasLetter := 0
	for _, r := range k {
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
		return fmt.Errorf("ong/middleware: secretKey should have at least %d digits", expected)
	}
	if hasSymbol < expected {
		return fmt.Errorf("ong/middleware: secretKey should have at least %d symbols", expected)
	}
	if hasLetter < expected {
		return fmt.Errorf("ong/middleware: secretKey should have at least %d letters", expected)
	}

	return nil
}

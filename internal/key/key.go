// Package key implements some common secure functionality.
package key

import (
	"fmt"
	"unicode"
)

// Some of the code here is inspired by(or taken from):
//   (a) https://github.com/wagslane/go-password-validator whose license(MIT) can be found here: https://github.com/wagslane/go-password-validator/blob/v0.3.0/LICENSE
//

// IsSecure checks that secretKey has at least some minimum desirable security properties.
func IsSecure(secretKey string) error {
	const (
		// Using a password like `4$kplejewjdsnv`(one number, one symbol & length 14)
		// would take about 90 million years to crack.
		// see:
		//   - https://www.passwordmonster.com/
		//   - https://thesecurityfactory.be/password-cracking-speed/
		minLen   = 14
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
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
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

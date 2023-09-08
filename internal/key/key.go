// Package key implements some common secure functionality.
package key

import (
	"errors"
	"fmt"
	"math"
	"unicode"
)

// Some of the code here is inspired by(or taken from):
//   (a) https://github.com/wagslane/go-password-validator whose license(MIT) can be found here:                                       https://github.com/wagslane/go-password-validator/blob/v0.3.0/LICENSE
//   (b) https://github.com/Xe/x/blob/v1.7.0/entropy/shannon.go whose license(Creative Commons Zero v1.0 Universal) can be found here: https://github.com/Xe/x/blob/v1.7.0/LICENSE
//   (c) https://github.com/danielmiessler/SecLists whose license(MIT) can be found here:                                              https://github.com/danielmiessler/SecLists/blob/2023.3/LICENSE
//

// IsSecure checks that secretKey has at least some minimum desirable security properties.
func IsSecure(secretKey string) error {
	const (
		// Using a password like `4$kplejewjdsnv`(one number, one symbol & length 14)
		// would take about 90 million years to crack.
		// see:
		//   - https://www.passwordmonster.com/
		//   - https://thesecurityfactory.be/password-cracking-speed/
		minLen          = 16
		maxLen          = 256
		minUniqueLen    = 10
		minEntropy      = 64
		minCombinations = 3
		expected        = 1
	)

	if len(secretKey) < minLen {
		return fmt.Errorf("ong: secretKey length is less than minimum required of %d", minLen)
	}
	if len(secretKey) > maxLen {
		return fmt.Errorf("ong: secretKey length is more than maximum required of %d", maxLen)
	}
	if entropy(secretKey) < minEntropy {
		return fmt.Errorf("ong: secretKey entropy is less than minimum required of %d", minEntropy)
	}

	hasDigit := 0
	hasSymbol := 0
	hasUpper := 0
	hasLower := 0
	for _, r := range secretKey {
		if unicode.IsDigit(r) {
			hasDigit = hasDigit + 1
		}
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			hasSymbol = hasSymbol + 1
		}
		if unicode.IsLetter(r) {
			if unicode.IsUpper(r) {
				hasUpper = hasUpper + 1
			}
			if unicode.IsLower(r) {
				hasLower = hasLower + 1
			}
		}
	}

	combinations := 0
	if hasDigit >= expected {
		combinations = combinations + 1
	}
	if hasSymbol > expected {
		combinations = combinations + 1
	}
	if hasUpper >= expected {
		combinations = combinations + 1
	}
	if hasLower >= expected {
		combinations = combinations + 1
	}

	if combinations < minCombinations {
		return errors.New("ong: secretKey should be a combination of digits, letters, symbols")
	}

	return nil
}

// entropy measures the entropy entropy of a string.
// See http://bearcave.com/misl/misl_tech/wavelets/compression/shannon.html for the algorithmic explanation.
func entropy(value string) (bits int) {
	frq := make(map[rune]float64)

	// frequency of characters
	for _, i := range value {
		frq[i]++
	}

	var sum float64

	for _, v := range frq {
		f := v / float64(len(value))
		sum += f * math.Log2(f)
	}

	bits = int(math.Ceil(sum*-1)) * len(value)
	return
}

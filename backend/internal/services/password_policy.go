package services

import (
	"errors"
	"unicode"
	"unicode/utf8"
)

// Account passwords (sign-up, reset and change) must have at least 8
// characters, an uppercase letter, a digit and a special character.
//
// This is a product decision against NIST SP 800-63B, which says verifiers
// shall not impose composition rules (ADR 0004). Link passwords keep only the
// length rule.
var (
	ErrPasswordMissingUppercase = errors.New("password has no uppercase letter")
	ErrPasswordMissingDigit     = errors.New("password has no digit")
	ErrPasswordMissingSpecial   = errors.New("password has no special character")
)

// validateAccountPassword checks a new account password. Uppercase is any
// Unicode uppercase letter (É counts), a digit is any decimal digit, and a
// special character is anything that is neither a letter, a digit nor
// whitespace. The web app applies the same rules.
func validateAccountPassword(password string) error {
	if utf8.RuneCountInString(password) < minPasswordCharacters {
		return ErrPasswordTooShort
	}
	if len(password) > maxPasswordBytes {
		return ErrPasswordTooLong
	}

	var hasUpper, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsLetter(r), unicode.IsSpace(r):
		default:
			hasSpecial = true
		}
	}

	switch {
	case !hasUpper:
		return ErrPasswordMissingUppercase
	case !hasDigit:
		return ErrPasswordMissingDigit
	case !hasSpecial:
		return ErrPasswordMissingSpecial
	}

	return nil
}

package services

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxFullNameCharacters = 100

var (
	ErrFullNameRequired = errors.New("full name is required")
	ErrFullNameTooLong  = errors.New("full name is too long")
	ErrInvalidFullName  = errors.New("full name has invalid characters")
)

// normalizeFullName trims the name and collapses inner whitespace, so
// "  Ana   Souza " is stored as "Ana Souza". Control and format characters are
// rejected because the name is shown back in the interface.
func normalizeFullName(name string) (string, error) {
	normalized := strings.Join(strings.Fields(name), " ")

	if normalized == "" {
		return "", ErrFullNameRequired
	}

	if utf8.RuneCountInString(normalized) > maxFullNameCharacters {
		return "", ErrFullNameTooLong
	}

	for _, r := range normalized {
		if r == utf8.RuneError || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", ErrInvalidFullName
		}
	}

	return normalized, nil
}

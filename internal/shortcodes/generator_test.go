package shortcodes_test

import (
	"strings"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/shortcodes"
)

const (
	expectedAlphabet   = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	expectedCodeLength = 10
)

func TestGeneratorGenerate(t *testing.T) {
	generator := shortcodes.Generator{}

	code, err := generator.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(code) != expectedCodeLength {
		t.Fatalf(
			"Generate() code length = %d, want %d",
			len(code),
			expectedCodeLength,
		)
	}

	for _, character := range code {
		if !strings.ContainsRune(expectedAlphabet, character) {
			t.Fatalf(
				"Generate() code = %q contains non-Base62 character %q",
				code,
				character,
			)
		}
	}
}

func TestGeneratorGenerateReturnsDifferentCodes(t *testing.T) {
	generator := shortcodes.Generator{}
	const generations = 1_000

	generatedCodes := make(map[string]struct{}, generations)

	for range generations {
		code, err := generator.Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}

		if _, exists := generatedCodes[code]; exists {
			t.Fatalf("Generate() returned duplicate code %q", code)
		}

		generatedCodes[code] = struct{}{}
	}
}

package shortcodes_test

import (
	"math"
	"strings"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/shortcodes"
)

const expectedAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func newGenerator(t *testing.T) *shortcodes.Generator {
	t.Helper()

	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	return generator
}

func TestGeneratorGenerate(t *testing.T) {
	generator := newGenerator(t)

	for _, id := range []int64{1, 2, 1_000, math.MaxInt64} {
		code, err := generator.Generate(id)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", id, err)
		}
		if len(code) < 6 {
			t.Errorf("Generate(%d) code length = %d, want at least 6", id, len(code))
		}
		if id == math.MaxInt64 && len(code) <= 6 {
			t.Errorf("Generate(%d) code length = %d, want more than 6", id, len(code))
		}

		for _, character := range code {
			if !strings.ContainsRune(expectedAlphabet, character) {
				t.Errorf("Generate(%d) code = %q contains non-Base62 character %q", id, code, character)
			}
		}
	}
}

func TestGeneratorGenerateIsDeterministic(t *testing.T) {
	firstGenerator := newGenerator(t)
	secondGenerator := newGenerator(t)

	firstCode, err := firstGenerator.Generate(42)
	if err != nil {
		t.Fatalf("first Generate(42) error = %v", err)
	}

	secondCode, err := secondGenerator.Generate(42)
	if err != nil {
		t.Fatalf("second Generate(42) error = %v", err)
	}

	if firstCode != secondCode {
		t.Errorf("Generate(42) = %q and %q with the same configuration", firstCode, secondCode)
	}
}

func TestGeneratorGenerateMatchesPinnedSqidsOutputs(t *testing.T) {
	generator := newGenerator(t)

	for _, test := range []struct {
		id   int64
		want string
	}{
		{id: 1, want: "UkLWZg"},
		{id: 42, want: "JgaEBg"},
	} {
		code, err := generator.Generate(test.id)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", test.id, err)
		}
		if code != test.want {
			t.Errorf("Generate(%d) = %q, want %q", test.id, code, test.want)
		}
	}
}

func TestGeneratorGenerateReturnsDifferentCodesForDifferentIDs(t *testing.T) {
	generator := newGenerator(t)
	const generations = 1_000

	generatedCodes := make(map[string]int64, generations)

	for id := int64(1); id <= generations; id++ {
		code, err := generator.Generate(id)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", id, err)
		}

		if previousID, exists := generatedCodes[code]; exists {
			t.Fatalf("Generate(%d) returned code %q already used by ID %d", id, code, previousID)
		}

		generatedCodes[code] = id
	}
}

func TestGeneratorGenerateRejectsInvalidIDs(t *testing.T) {
	generator := newGenerator(t)

	for _, id := range []int64{0, -1, math.MinInt64} {
		if _, err := generator.Generate(id); err == nil {
			t.Errorf("Generate(%d) error = nil, want an error", id)
		}
	}
}

func TestGeneratorGenerateRejectsUninitializedGenerator(t *testing.T) {
	var zeroValue shortcodes.Generator
	if _, err := zeroValue.Generate(1); err == nil {
		t.Error("zero-value Generator.Generate(1) error = nil, want an error")
	}

	var nilGenerator *shortcodes.Generator
	if _, err := nilGenerator.Generate(1); err == nil {
		t.Error("nil Generator.Generate(1) error = nil, want an error")
	}
}

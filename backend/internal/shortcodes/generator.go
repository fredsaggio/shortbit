package shortcodes

import (
	"errors"
	"fmt"

	"github.com/sqids/sqids-go"
)

// reservedCodes are paths of the web app that live at the same level as the
// short links (GET /{code}). Only those of at least six Base62 characters could
// ever be generated. Matching is exact, since paths are case-sensitive.
var reservedCodes = map[string]struct{}{
	"entrar":   {},
	"cadastro": {},
	"painel":   {},
	"assets":   {},
}

type Generator struct {
	encoder *sqids.Sqids
}

func NewGenerator() (*Generator, error) {
	encoder, err := sqids.New(sqids.Options{MinLength: 6})
	if err != nil {
		return nil, fmt.Errorf("configure shortcode generator: %w", err)
	}

	return &Generator{encoder: encoder}, nil
}

func (g *Generator) Generate(id int64) (string, error) {
	if g == nil || g.encoder == nil {
		return "", errors.New("shortcode generator is not initialized")
	}

	if id <= 0 {
		return "", fmt.Errorf("invalid URL id: %d", id)
	}

	code, err := g.encoder.Encode([]uint64{uint64(id)})

	if err != nil {
		return "", fmt.Errorf("encode URL id %d: %w", id, err)
	}

	if IsReserved(code) {
		// A different input gives a different, still unique code. Codes are
		// stored, never decoded, so the extra number costs nothing.
		code, err = g.encoder.Encode([]uint64{uint64(id), 0})
		if err != nil {
			return "", fmt.Errorf("encode URL id %d: %w", id, err)
		}
	}

	return code, nil
}

// IsReserved reports whether code is a web app path, such as "painel".
func IsReserved(code string) bool {
	_, reserved := reservedCodes[code]
	return reserved
}

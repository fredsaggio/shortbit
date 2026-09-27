package shortcodes

import (
	"errors"
	"fmt"

	"github.com/sqids/sqids-go"
)

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

	return code, nil
}

package shortcodes

import (
	"crypto/rand"
	"fmt"
)

const (
	alphabet   = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	codeLength = 10

	// Tem que ser 248, números após isso dariam vantagem para cair em letras específicas.
	maxUnbiasedByte = 256 - (256 % len(alphabet))
)

type Generator struct{}

func (Generator) Generate() (string, error) {
	code := make([]byte, codeLength)

	var randomBytes [codeLength]byte

	position := 0

	for position < codeLength {
		remaining := codeLength - position

		// Gera bytes aleatórios e aloca no array randomBytes
		if _, err := rand.Read(randomBytes[:remaining]); err != nil {
			return "", fmt.Errorf("generate random short code: %w", err)
		}

		// Verifica cada byte, transforma em um número entre 0 e 61 que é o index de alphabet e aloca em code para criar o shortcode.
		for _, randomByte := range randomBytes[:remaining] {
			if int(randomByte) >= maxUnbiasedByte {
				continue
			}

			index := int(randomByte) % len(alphabet)
			code[position] = alphabet[index]
			position++

			if position == codeLength {
				break
			}
		}
	}

	return string(code), nil
}

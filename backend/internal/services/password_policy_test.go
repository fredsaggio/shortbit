package services

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateAccountPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     error
	}{
		{name: "meets every rule", password: "Senha-segura1", want: nil},
		{name: "accented uppercase counts", password: "Érica#2026", want: nil},
		{name: "emoji counts as special", password: "Senha1234😀", want: nil},
		{name: "too short", password: "Ab1!", want: ErrPasswordTooShort},
		{name: "counts characters, not bytes", password: "Éééééé1!", want: nil},
		{name: "too long", password: "Aa1!" + strings.Repeat("a", 1021), want: ErrPasswordTooLong},
		{name: "no uppercase", password: "senha-segura1", want: ErrPasswordMissingUppercase},
		{name: "no digit", password: "Senha-segura", want: ErrPasswordMissingDigit},
		{name: "no special", password: "SenhaSegura1", want: ErrPasswordMissingSpecial},
		{name: "space is not special", password: "Senha Segura1", want: ErrPasswordMissingSpecial},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateAccountPassword(tt.password); !errors.Is(got, tt.want) {
				t.Errorf("validateAccountPassword(%q) = %v, want %v", tt.password, got, tt.want)
			}
		})
	}
}

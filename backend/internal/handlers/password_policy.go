package handlers

import (
	"errors"

	"github.com/fredsaggio/url-shortener/internal/services"
)

var passwordPolicyMessages = []struct {
	err     error
	message string
}{
	{services.ErrPasswordTooShort, "senha muito curta"},
	{services.ErrPasswordTooLong, "senha muito longa"},
	{services.ErrPasswordMissingUppercase, "a senha precisa ter pelo menos uma letra maiúscula"},
	{services.ErrPasswordMissingDigit, "a senha precisa ter pelo menos um número"},
	{services.ErrPasswordMissingSpecial, "a senha precisa ter pelo menos um caractere especial"},
}

// isPasswordPolicyError reports whether err is a rule of the account password
// policy, shared by sign-up, password reset and password change.
func isPasswordPolicyError(err error) bool {
	_, ok := passwordPolicyMessageFor(err)
	return ok
}

func passwordPolicyMessage(err error) string {
	message, _ := passwordPolicyMessageFor(err)
	return message
}

func passwordPolicyMessageFor(err error) (string, bool) {
	for _, rule := range passwordPolicyMessages {
		if errors.Is(err, rule.err) {
			return rule.message, true
		}
	}
	return "", false
}

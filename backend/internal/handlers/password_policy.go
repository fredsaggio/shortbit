package handlers

import (
	"errors"
	"net/http"

	"github.com/fredsaggio/url-shortener/internal/problem"

	"github.com/fredsaggio/url-shortener/internal/services"
)

var passwordPolicyErrors = []struct {
	err     error
	code    string
	message string
}{
	{services.ErrPasswordTooShort, "password_too_short", "senha muito curta"},
	{services.ErrPasswordTooLong, "password_too_long", "senha muito longa"},
	{services.ErrPasswordMissingUppercase, "password_missing_uppercase", "a senha precisa ter pelo menos uma letra maiúscula"},
	{services.ErrPasswordMissingDigit, "password_missing_digit", "a senha precisa ter pelo menos um número"},
	{services.ErrPasswordMissingSpecial, "password_missing_special", "a senha precisa ter pelo menos um caractere especial"},
}

// isPasswordPolicyError reports whether err is a rule of the account password
// policy, shared by sign-up, password reset and password change.
func isPasswordPolicyError(err error) bool {
	_, _, ok := passwordPolicyError(err)
	return ok
}

// passwordPolicyError returns the problem code and message of a password
// policy rule.
func passwordPolicyError(err error) (code, message string, ok bool) {
	for _, rule := range passwordPolicyErrors {
		if errors.Is(err, rule.err) {
			return rule.code, rule.message, true
		}
	}
	return "", "", false
}

func writePasswordPolicyError(w http.ResponseWriter, r *http.Request, err error) {
	code, message, _ := passwordPolicyError(err)
	problem.Write(w, r, http.StatusBadRequest, code, message)
}

package handlers

import (
	"errors"

	"github.com/fredsaggio/url-shortener/internal/services"
)

func fullNameErrorMessage(err error) string {
	switch {
	case errors.Is(err, services.ErrFullNameTooLong):
		return "nome completo muito longo; use até 100 caracteres"
	case errors.Is(err, services.ErrInvalidFullName):
		return "nome completo contém caracteres inválidos"
	default:
		return "informe o nome completo"
	}
}

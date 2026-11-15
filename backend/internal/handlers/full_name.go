package handlers

import (
	"errors"
	"net/http"

	"github.com/fredsaggio/url-shortener/internal/problem"
	"github.com/fredsaggio/url-shortener/internal/services"
)

func writeFullNameError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, services.ErrFullNameTooLong):
		problem.Write(w, r, http.StatusBadRequest, "full_name_too_long", "nome completo muito longo; use até 100 caracteres")
	case errors.Is(err, services.ErrInvalidFullName):
		problem.Write(w, r, http.StatusBadRequest, "invalid_full_name", "nome completo contém caracteres inválidos")
	default:
		problem.Write(w, r, http.StatusBadRequest, "full_name_required", "informe o nome completo")
	}
}

package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/fredsaggio/url-shortener/internal/problem"
)

// decodeSingleJSON reads exactly one JSON object with no unknown fields into
// target. On failure it answers the request and returns false: 413 when the
// body went past the global size limit (middleware.LimitRequestBody), 400 otherwise.
func decodeSingleJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		writeDecodeError(w, r, err, "corpo da requisição inválido")
		return false
	}
	// Anything after the object, even within the limit, is rejected.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeDecodeError(w, r, err, "corpo da requisição deve conter exatamente um objeto JSON")
		return false
	}
	return true
}

func writeDecodeError(w http.ResponseWriter, r *http.Request, err error, detail string) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		problem.Write(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "corpo da requisição grande demais")
		return
	}
	problem.Write(w, r, http.StatusBadRequest, "invalid_body", detail)
}

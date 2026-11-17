package models_test

import (
	"bytes"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/models"
)

func TestVisitDetailsValid(t *testing.T) {
	hash := bytes.Repeat([]byte{1}, models.VisitorHashSize)
	tests := []struct {
		name    string
		details models.VisitDetails
		want    bool
	}{
		{"nothing known", models.VisitDetails{}, true},
		{"person with country and visitor", models.VisitDetails{Country: "BR", Visitor: hash}, true},
		{"robot", models.VisitDetails{Robot: true}, true},
		{"robot with country", models.VisitDetails{Robot: true, Country: "BR"}, false},
		{"robot with visitor", models.VisitDetails{Robot: true, Visitor: hash}, false},
		{"lowercase country", models.VisitDetails{Country: "br"}, false},
		{"three letters", models.VisitDetails{Country: "BRA"}, false},
		{"short visitor", models.VisitDetails{Visitor: hash[:8]}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.details.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

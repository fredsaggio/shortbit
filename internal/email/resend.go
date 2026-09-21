package email

import (
	"context"
	"fmt"

	"github.com/resend/resend-go/v4"
)

type ResendSender struct {
	client *resend.Client
	from   string
}

func NewResendSender(apiKey, from string) *ResendSender {
	return &ResendSender{
		client: resend.NewClient(apiKey),
		from:   from,
	}
}

func (rs *ResendSender) SendPasswordRegistrationCode(ctx context.Context, email, code string) error {
	params := &resend.SendEmailRequest{
		From:    rs.from,
		To:      []string{email},
		Subject: "Confirme seu cadastro",
		Text:    fmt.Sprintf("Seu código de confirmação é: %s\n\nSe você não iniciou este cadastro, ignore este email.", code),
	}

	if _, err := rs.client.Emails.SendWithContext(ctx, params); err != nil {
		return fmt.Errorf("send password registration code with Resend: %w", err)
	}

	return nil
}

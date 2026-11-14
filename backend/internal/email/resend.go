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

func (rs *ResendSender) SendPasswordResetCode(ctx context.Context, email, code string) error {
	params := &resend.SendEmailRequest{
		From:    rs.from,
		To:      []string{email},
		Subject: "Código para redefinir sua senha",
		Text: fmt.Sprintf(
			"Seu código para redefinir a senha é: %s\n\n"+
				"Se você não solicitou a redefinição, ignore este email.",
			code,
		),
	}

	if _, err := rs.client.Emails.SendWithContext(ctx, params); err != nil {
		return fmt.Errorf("send password reset code with Resend: %w", err)
	}

	return nil
}

func (rs *ResendSender) SendPasswordChangedNotice(ctx context.Context, email string) error {
	params := &resend.SendEmailRequest{
		From:    rs.from,
		To:      []string{email},
		Subject: "Sua senha foi alterada",
		Text:    "A senha da sua conta foi alterada. Se não foi você, entre em contato com o suporte.",
	}
	if _, err := rs.client.Emails.SendWithContext(ctx, params); err != nil {
		return fmt.Errorf("send password changed notice with Resend: %w", err)
	}
	return nil
}

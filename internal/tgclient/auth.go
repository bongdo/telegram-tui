package tgclient

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

type authenticator struct {
	b *Bridge
}

func (a authenticator) Phone(ctx context.Context) (string, error) {
	if phone := strings.TrimSpace(os.Getenv("TG_PHONE")); phone != "" {
		return phone, nil
	}
	return a.b.ask(ctx, AuthPrompt{
		Kind: AuthPhone,
		Hint: "International format, for example +1 202 555 0143",
	})
}

func (a authenticator) Code(ctx context.Context, sent *tg.AuthSentCode) (string, error) {
	hint := "Enter the code from Telegram"
	switch t := sent.Type.(type) {
	case *tg.AuthSentCodeTypeApp:
		hint = fmt.Sprintf("Code sent in the Telegram app (%d digits)", t.Length)
	case *tg.AuthSentCodeTypeSMS:
		hint = fmt.Sprintf("Code sent by SMS (%d digits)", t.Length)
	case *tg.AuthSentCodeTypeCall:
		hint = fmt.Sprintf("Telegram will call you with a %d-digit code", t.Length)
	case *tg.AuthSentCodeTypeFlashCall:
		hint = "Answer the flash call and enter the code"
	case *tg.AuthSentCodeTypeEmailCode:
		hint = "Code sent to your email"
	}
	return a.b.ask(ctx, AuthPrompt{Kind: AuthCode, Hint: hint})
}

func (a authenticator) Password(ctx context.Context) (string, error) {
	return a.b.ask(ctx, AuthPrompt{
		Kind: AuthPassword,
		Hint: "Two-factor authentication password",
	})
}

func (a authenticator) AcceptTermsOfService(context.Context, tg.HelpTermsOfService) error {
	return errors.New("account sign-up is not supported; sign in to an existing account")
}

func (a authenticator) SignUp(context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("account sign-up is not supported")
}

var _ auth.UserAuthenticator = authenticator{}

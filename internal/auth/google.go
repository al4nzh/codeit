package auth

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"google.golang.org/api/idtoken"
)

type GoogleIdentity struct {
	Email   string
	Name    string
	Picture string
}

type GoogleTokenVerifier struct {
	clientID string
}

func NewGoogleTokenVerifier(clientID string) *GoogleTokenVerifier {
	return &GoogleTokenVerifier{clientID: strings.TrimSpace(clientID)}
}

func (v *GoogleTokenVerifier) VerifyIDToken(ctx context.Context, rawToken string) (*GoogleIdentity, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || v.clientID == "" {
		return nil, errors.New("invalid google token input")
	}
	payload, err := idtoken.Validate(ctx, rawToken, v.clientID)
	if err != nil {
		return nil, err
	}

	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)
	if email == "" {
		return nil, errors.New("google token missing email")
	}
	if !isGoogleEmailVerifiedClaim(payload.Claims["email_verified"]) {
		return nil, errors.New("google email not verified")
	}

	return &GoogleIdentity{
		Email:   email,
		Name:    name,
		Picture: picture,
	}, nil
}

// isGoogleEmailVerifiedClaim returns true if the claim is absent or clearly verified.
// Google may send email_verified as bool or string depending on account / token path.
func isGoogleEmailVerifiedClaim(v any) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		if s == "" {
			return true
		}
		b, err := strconv.ParseBool(s)
		if err != nil {
			return true
		}
		return b
	case float64:
		if x == 1 {
			return true
		}
		if x == 0 {
			return false
		}
		return true
	default:
		return true
	}
}

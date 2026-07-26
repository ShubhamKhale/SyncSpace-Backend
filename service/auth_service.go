package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service/database"
	"syncspace-backend/service/session"
)

// AuthService handles registration, sign-in, JWT issuance, and session key management.
type AuthService struct {
	userRepo   *database.UserRepo
	orgRepo    *database.OrgRepo
	inviteRepo *database.InviteTokenRepo
	store      *session.Store
	jwtSecret  string
}

// NewAuthService creates an AuthService backed by a user repo, org repo, invite token repo, and session store.
func NewAuthService(
	userRepo *database.UserRepo,
	orgRepo *database.OrgRepo,
	inviteRepo *database.InviteTokenRepo,
	store *session.Store,
	jwtSecret string,
) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		orgRepo:    orgRepo,
		inviteRepo: inviteRepo,
		store:      store,
		jwtSecret:  jwtSecret,
	}
}

// Register creates a new user account: hashes the password with bcrypt, persists
// the user, optionally joins an org via invite token, and returns a JWT + session key.
func (s *AuthService) Register(ctx context.Context, name, email, password, inviteToken string) (token, sessionKey string, user *model.AuthUser, err error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", "", nil, errs.Internal("failed to hash password")
	}

	now := time.Now()
	newUser := &model.User{
		ID:           utils.NewUUID(),
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err = s.userRepo.InsertUser(ctx, newUser); err != nil {
		return "", "", nil, err
	}

	// Handle invite token — auto-join org if valid token provided.
	if inviteToken != "" {
		invite, invErr := s.inviteRepo.GetValidInviteToken(ctx, inviteToken)
		if invErr != nil {
			return "", "", nil, invErr // 400 BadRequest
		}

		member := &model.OrgMember{
			OrgID:     invite.OrgID,
			UserID:    newUser.ID,
			Role:      invite.Role,
			Status:    "active",
			InvitedBy: invite.InvitedBy,
			JoinedAt:  now,
		}
		if insertErr := s.orgRepo.InsertOrgMember(ctx, member); insertErr != nil {
			return "", "", nil, insertErr
		}
		_ = s.inviteRepo.MarkInviteUsed(ctx, inviteToken)
	}

	token, err = s.issueJWT(newUser.ID)
	if err != nil {
		return "", "", nil, err
	}

	sessionKey, err = s.IssueSessionKey(newUser.ID)
	if err != nil {
		return "", "", nil, err
	}

	user, err = s.userRepo.GetUserWithOrg(ctx, newUser.ID)
	if err != nil {
		return "", "", nil, err
	}

	return token, sessionKey, user, nil
}

// SignIn validates the email/password pair and returns a signed JWT and AES session key.
// Always returns errs.Unauthorized on any credential failure to avoid leaking
// whether the email exists.
func (s *AuthService) SignIn(ctx context.Context, email, password string) (token, sessionKey string, user *model.AuthUser, err error) {
	existing, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", "", nil, errs.Unauthorized("invalid email or password")
	}

	if err = bcrypt.CompareHashAndPassword([]byte(existing.PasswordHash), []byte(password)); err != nil {
		return "", "", nil, errs.Unauthorized("invalid email or password")
	}

	token, err = s.issueJWT(existing.ID)
	if err != nil {
		return "", "", nil, err
	}

	sessionKey, err = s.IssueSessionKey(existing.ID)
	if err != nil {
		return "", "", nil, err
	}

	user, err = s.userRepo.GetUserWithOrg(ctx, existing.ID)
	if err != nil {
		return "", "", nil, err
	}

	return token, sessionKey, user, nil
}

// IssueSessionKey generates a fresh AES-256 session key for userID, stores it
// in the session store, and returns it base64-encoded for the frontend.
func (s *AuthService) IssueSessionKey(userID string) (string, error) {
	key, err := s.store.GenerateKey(userID)
	if err != nil {
		return "", fmt.Errorf("auth: issue session key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// RevokeSessionKey removes the user's AES session key on logout.
func (s *AuthService) RevokeSessionKey(userID string) {
	s.store.DeleteKey(userID)
}

// issueJWT signs a HS256 JWT for userID with a 24-hour expiry.
func (s *AuthService) issueJWT(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", errs.Internal("failed to sign token")
	}
	return signed, nil
}

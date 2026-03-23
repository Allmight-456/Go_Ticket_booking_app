package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Allmight-456/ticketflow/internal/domain"
	"github.com/Allmight-456/ticketflow/internal/repository/postgres"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users     *postgres.UserRepo
	jwtSecret string
	jwtExpiry time.Duration
}

func NewAuthService(users *postgres.UserRepo, jwtSecret string, jwtExpiry time.Duration) *AuthService {
	return &AuthService{users: users, jwtSecret: jwtSecret, jwtExpiry: jwtExpiry}
}

// Register creates a new user and returns the user + signed JWT token.
func (s *AuthService) Register(ctx context.Context, email, password, firstName, lastName string) (*domain.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := validateEmail(email); err != nil {
		return nil, "", err
	}
	if len(password) < 8 {
		return nil, "", errors.New("password must be at least 8 characters")
	}
	if len(firstName) < 2 || len(lastName) < 2 {
		return nil, "", errors.New("first and last name must be at least 2 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		Email:        email,
		PasswordHash: string(hash),
		FirstName:    firstName,
		LastName:     lastName,
		Role:         domain.RoleUser,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, "", err
	}

	token, err := s.issueToken(user)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

// Login verifies credentials and returns the user + a fresh JWT.
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, "", domain.ErrInvalidCredentials
		}
		return nil, "", err
	}

	if !user.IsActive {
		return nil, "", domain.ErrUserInactive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", domain.ErrInvalidCredentials
	}

	token, err := s.issueToken(user)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

// ValidateToken parses and verifies a JWT, returning the embedded claims.
func (s *AuthService) ValidateToken(tokenString string) (*domain.UserClaims, error) {
	claims := &domain.UserClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	return claims, nil
}

func (s *AuthService) issueToken(u *domain.User) (string, error) {
	claims := domain.UserClaims{
		UserID: u.ID,
		Email:  u.Email,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtExpiry)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func validateEmail(email string) error {
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return errors.New("invalid email address")
	}
	return nil
}

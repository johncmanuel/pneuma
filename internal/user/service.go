package user

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"pneuma/internal/models"
	"pneuma/internal/store/sqlite"
)

var (
	ErrUserExists    = errors.New("username already taken")
	ErrWrongPassword = errors.New("invalid credentials")
)

// Service manages user accounts and devices.
type Service struct {
	store *sqlite.Store
}

// New creates a user Service.
func New(store *sqlite.Store) *Service {
	return &Service{store: store}
}

// Register creates a new user. The first registered user becomes admin.
func (s *Service) Register(ctx context.Context, username, password string) (*models.User, error) {
	existing, err := s.store.UserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	count, err := s.store.CountUsers(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	u := &models.User{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: string(hash),
		IsAdmin:      count == 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.store.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Login verifies credentials and returns the User.
func (s *Service) Login(ctx context.Context, username, password string) (*models.User, error) {
	u, err := s.store.UserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrWrongPassword
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, ErrWrongPassword
	}
	return u, nil
}

// GetByID returns a user by ID.
func (s *Service) GetByID(ctx context.Context, id string) (*models.User, error) {
	return s.store.UserByID(ctx, id)
}

// ChangePassword updates a user password (admin-style, no old password check).
func (s *Service) ChangePassword(ctx context.Context, userID, newPwd string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.store.UpdateUserPassword(ctx, userID, string(hash))
}

// RegisterDevice registers or updates a device.
func (s *Service) RegisterDevice(ctx context.Context, userID, deviceName string) (*models.Device, error) {
	now := time.Now()
	d := &models.Device{
		ID:         uuid.NewString(),
		UserID:     userID,
		Name:       deviceName,
		LastSeenAt: &now,
		CreatedAt:  now,
	}
	if err := s.store.UpsertDevice(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// TouchDevice updates the last_seen_at timestamp.
func (s *Service) TouchDevice(ctx context.Context, deviceID string) error {
	return s.store.TouchDevice(ctx, deviceID)
}

// Devices returns all devices for a user.
func (s *Service) Devices(ctx context.Context, userID string) ([]*models.Device, error) {
	return s.store.DevicesByUser(ctx, userID)
}

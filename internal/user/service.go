package user

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"pneuma/internal/models"
	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/serverdb"
)

var (
	ErrUserExists    = errors.New("username already taken")
	ErrWrongPassword = errors.New("invalid credentials")
	ErrNotFound      = errors.New("user not found")
	ErrSelfDelete    = errors.New("cannot delete yourself")
)

// Service manages user accounts and devices.
type Service struct {
	q *serverdb.Queries
}

// New creates a user Service.
func New(q *serverdb.Queries) *Service {
	return &Service{q: q}
}

// Register creates a new user. The first registered user becomes admin.
func (s *Service) Register(ctx context.Context, username, password string) (*models.User, error) {
	_, err := s.q.UserByUsername(ctx, username)
	if err == nil {
		return nil, ErrUserExists
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	count, err := s.q.CountUsers(ctx)
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
	if err := s.q.CreateUser(ctx, serverdb.CreateUserParams{
		ID:           u.ID,
		Username:     u.Username,
		PasswordHash: u.PasswordHash,
		IsAdmin:      dbconv.BoolInt(u.IsAdmin),
		CanUpload:    dbconv.BoolInt(u.CanUpload),
		CanEdit:      dbconv.BoolInt(u.CanEdit),
		CanDelete:    dbconv.BoolInt(u.CanDelete),
		CreatedAt:    dbconv.FormatTime(u.CreatedAt),
		UpdatedAt:    dbconv.FormatTime(u.UpdatedAt),
	}); err != nil {
		return nil, err
	}
	return u, nil
}

// Login verifies credentials and returns the User.
func (s *Service) Login(ctx context.Context, username, password string) (*models.User, error) {
	row, err := s.q.UserByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWrongPassword
	}
	if err != nil {
		return nil, err
	}
	u := dbconv.UserToModel(row)
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, ErrWrongPassword
	}
	return u, nil
}

// GetByID returns a user by ID.
func (s *Service) GetByID(ctx context.Context, id string) (*models.User, error) {
	row, err := s.q.UserByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return dbconv.UserToModel(row), nil
}

// ChangePassword updates a user password (admin-style, no old password check).
func (s *Service) ChangePassword(ctx context.Context, userID, newPwd string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.q.UpdateUserPassword(ctx, serverdb.UpdateUserPasswordParams{
		PasswordHash: string(hash),
		UpdatedAt:    dbconv.FormatTime(time.Now()),
		ID:           userID,
	})
}

// RegisterDevice registers or updates a device.
func (s *Service) RegisterDevice(ctx context.Context, userID, deviceName string) (*models.Device, error) {
	now := time.Now()
	nowStr := dbconv.FormatTime(now)
	d := &models.Device{
		ID:         uuid.NewString(),
		UserID:     userID,
		Name:       deviceName,
		LastSeenAt: &now,
		CreatedAt:  now,
	}
	if err := s.q.UpsertDevice(ctx, serverdb.UpsertDeviceParams{
		ID:         d.ID,
		UserID:     d.UserID,
		Name:       d.Name,
		LastSeenAt: sql.NullString{String: nowStr, Valid: true},
		CreatedAt:  nowStr,
	}); err != nil {
		return nil, err
	}
	return d, nil
}

// TouchDevice updates the last_seen_at timestamp.
func (s *Service) TouchDevice(ctx context.Context, deviceID string) error {
	return s.q.TouchDevice(ctx, serverdb.TouchDeviceParams{
		LastSeenAt: sql.NullString{String: dbconv.FormatTime(time.Now()), Valid: true},
		ID:         deviceID,
	})
}

// Devices returns all devices for a user.
func (s *Service) Devices(ctx context.Context, userID string) ([]*models.Device, error) {
	rows, err := s.q.DevicesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return dbconv.DevicesToModels(rows), nil
}

// ListUsers returns all registered users.
func (s *Service) ListUsers(ctx context.Context) ([]*models.User, error) {
	rows, err := s.q.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	return dbconv.UsersToModels(rows), nil
}

// UpdatePermissions sets the permission flags for a user.
func (s *Service) UpdatePermissions(ctx context.Context, userID string, canUpload, canEdit, canDelete bool) error {
	_, err := s.q.UserByID(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return s.q.UpdateUserPermissions(ctx, serverdb.UpdateUserPermissionsParams{
		CanUpload: dbconv.BoolInt(canUpload),
		CanEdit:   dbconv.BoolInt(canEdit),
		CanDelete: dbconv.BoolInt(canDelete),
		UpdatedAt: dbconv.FormatTime(time.Now()),
		ID:        userID,
	})
}

// DeleteUser removes a user. callerID is the user performing the action —
// a user cannot delete themselves.
func (s *Service) DeleteUser(ctx context.Context, callerID, targetID string) error {
	if callerID == targetID {
		return ErrSelfDelete
	}
	_, err := s.q.UserByID(ctx, targetID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return s.q.DeleteUser(ctx, targetID)
}

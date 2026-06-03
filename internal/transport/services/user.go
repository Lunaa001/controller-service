package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"service-controller-notebookum/internal/transport/upstream"
)

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type UserService struct {
	client *upstream.Client
}

func NewUserService(baseURL string, timeout time.Duration) *UserService {
	return &UserService{
		client: upstream.New(baseURL, timeout),
	}
}

// Create crea un nuevo usuario en el servicio de usuarios
func (s *UserService) Create(ctx context.Context, name, email string, headers http.Header) (*User, error) {
	payload, _ := json.Marshal(map[string]string{
		"name":  name,
		"email": email,
	})

	status, body, _, err := s.client.Request(http.MethodPost, "/api/v1/users", payload, headers)
	if err != nil {
		return nil, err
	}

	if status != http.StatusCreated && status != http.StatusOK {
		return nil, errors.New("failed to create user")
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// Get obtiene un usuario por ID
func (s *UserService) Get(ctx context.Context, userID string, headers http.Header) (*User, error) {
	status, body, _, err := s.client.Request(http.MethodGet, "/api/v1/users/"+userID, nil, headers)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, ErrUserNotFound
	}

	if status != http.StatusOK {
		return nil, errors.New("failed to get user")
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

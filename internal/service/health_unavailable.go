package service

import "context"

func (s *Service) HealthCheck(ctx context.Context) error {
	return s.repo.Ping(ctx)
}

type ServiceUnavailableError struct{ Message string }

func (e ServiceUnavailableError) Error() string { return e.Message }

func NewServiceUnavailableError(message string) ServiceUnavailableError {
	return ServiceUnavailableError{Message: message}
}

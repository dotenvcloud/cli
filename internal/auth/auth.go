package auth

// AuthProvider handles authentication flows
type AuthProvider interface {
	Login() error
	Refresh() error
	GetToken() (string, error)
}

// TODO: This will be implemented by Agent 5

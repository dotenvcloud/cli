package auth

// Provider handles authentication flows.
type Provider interface {
	Login() error
	Refresh() error
	GetToken() (string, error)
}

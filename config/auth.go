package config

// GetAuthScheme returns the auth scheme (read at call time)
func GetAuthScheme() string {
	return envOr("AUTH_SCHEME", "web")
}

// GetAuthModel returns the auth model (read at call time)
func GetAuthModel() string {
	return envOr("AUTH_MODEL", "User")
}

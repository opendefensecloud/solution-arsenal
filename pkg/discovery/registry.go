package discovery

// Registry is a struct representing an OCI registry.
type Registry struct {
	// PlainHTTP is a boolean flag indicating whether the repository was discovered using plain HTTP
	PlainHTTP bool
	// Hostname is the hostname of the registry
	Hostname string
	// Credentials is a struct containing credentials for accessing the registry
	Credentials RegistryCredentials
}

// RegistryCredentials represents the credentials required to access an OCI registry.
type RegistryCredentials struct {
	// Username is the username used to authenticate with the registry
	Username string
	// Password is the password used to authenticate with the registry
	Password string
}

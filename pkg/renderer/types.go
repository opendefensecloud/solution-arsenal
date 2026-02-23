package renderer

type AuthenticationType string

const (
	AuthenticationTypeBasic            AuthenticationType = "basic"
	AuthenticationTypeDockerConfigJson AuthenticationType = "dockerconfigjson"
)

type PushOptions struct {
	AuthenticationType
	ReferenceURL    string
	CredentialsFile string
	Username        string
	Password        string
	PlainHTTP       bool
}

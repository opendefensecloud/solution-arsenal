package types

type Config struct {
	Registry Registry `yaml:"registry"`
}

type Registry struct {
	Name         string   `yaml:"name"`
	URL          string   `yaml:"url"`
	Flavor       string   `yaml:"flavor"`
	Webhook      *Webhook `yaml:"webhook"`
	ScanInterval string   `yaml:"scanInterval" default:"1h"`
}

type Webhook struct {
	Path string `yaml:"path"`
}

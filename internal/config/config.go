package config

import (
	"github.com/kelseyhightower/envconfig"
)

// Config holds the configuration values for all the things
type Config struct {
	Twilio struct {
		Sender struct {
			PhoneNumber string
			AccountSID  string
			AuthToken   string
		}
		Receiver struct {
			Port int `default:"5000"`
		}
	}
}

// Parse loads values from the environment into the receiver
func (c *Config) Parse() error {
	return envconfig.Process("", c)
}

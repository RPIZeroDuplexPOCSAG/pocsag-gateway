package settings

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// App defines the Application Settings structure
type App struct {
	AMQPURL		string

	FreqOffset	int

	TXBaud		int
	TXFreq		int
	TXPower		int

	RXBaud		int
	RXFreq		int

	InvertBits	bool
	DutyCycle	int
}

// LoadSettings will pull the application config from the environment, or from
// a .env file
func LoadSettings() (config *App, err error) {
	config = &App{}

	if err = godotenv.Load(); err != nil {
		// We don't care if an .env is missing, it will be in prod.
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if err = envconfig.Process("pocgw", config); err != nil {
		return nil, err
	}

	return config, nil
}
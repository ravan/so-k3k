package config

import (
	"github.com/alecthomas/kingpin"
	"github.com/go-playground/validator/v10"
	"github.com/ravan/stackstate-client/stackstate"
	"github.com/ravan/stackstate-client/stackstate/receiver"
	"github.com/spf13/viper"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Configuration struct {
	SuseObservability stackstate.StackState `mapstructure:"suseobservability" validate:"required"`
	Instance          receiver.Instance     `mapstructure:"instance" validate:"required"`
}

func GetConfig() (*Configuration, error) {
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		cf := kingpin.Flag("config-file", "config file").Short('c').ExistingFile()
		if *cf != "" {
			configFile = *cf
		}
	}
	c := &Configuration{Instance: receiver.Instance{}}
	v := viper.New()
	v.SetDefault("suseobservability.api_url", "")
	v.SetDefault("suseobservability.api_key", "")
	v.SetDefault("suseobservability.api_token", "")
	v.SetDefault("instance.type", "k3k")
	v.SetDefault("instance.url", "vcluster")

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if configFile != "" {
		d, f := path.Split(configFile)
		if d == "" {
			d = "."
		}
		v.SetConfigName(f[0 : len(f)-len(filepath.Ext(f))])
		v.AddConfigPath(d)
		err := v.ReadInConfig()
		if err != nil {
			slog.Error("Error when reading config file.", slog.Any("error", err))
		}
	}

	if err := v.Unmarshal(c); err != nil {
		slog.Error("Error unmarshalling config", slog.Any("err", err))
		return nil, err
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

package flags

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type FlagBinding struct {
	FlagName string
	ViperKey string
	EnvVar   string // optional, for backwards compatibility
}

func AddAndBindFlags(flags *pflag.FlagSet, bindings []FlagBinding) error {
	for _, b := range bindings {
		if err := viper.BindPFlag(b.ViperKey, flags.Lookup(b.FlagName)); err != nil {
			return err
		}
		if b.EnvVar != "" {
			if err := viper.BindEnv(b.ViperKey, b.EnvVar); err != nil {
				return err
			}
		}
	}

	return nil
}

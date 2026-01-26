package config

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/viper"
)

var log = logging.Logger("config")

// Normalizable allows configs to adjust legacy/default values prior to validation.
type Normalizable interface {
	Normalize()
}

func Load[T Validatable]() (T, error) {
	var out T

	SetDefaults()

	if err := viper.Unmarshal(&out); err != nil {
		return out, err
	}
	if n, ok := any(&out).(Normalizable); ok {
		n.Normalize()
	}
	if err := out.Validate(); err != nil {
		return out, err
	}

	return out, nil
}

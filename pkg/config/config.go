package config

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/viper"
)

var log = logging.Logger("config")

func Load[T Validatable]() (T, error) {
	var out T
	if err := viper.Unmarshal(&out); err != nil {
		return out, err
	}
	if err := out.Validate(); err != nil {
		return out, err
	}

	return out, nil
}

package config

import (
	"fmt"

	"github.com/spf13/viper"
)

func Load[T Validatable]() (T, error) {
	var out T
	if err := viper.Unmarshal(&out); err != nil {
		return out, err
	}
	if err := viper.GetViper().WriteConfigAs("testing.toml"); err != nil {
		panic(err)
	}

	fmt.Printf("%+v", out)
	if err := out.Validate(); err != nil {
		return out, err
	}

	return out, nil
}

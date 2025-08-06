package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/viper"
)

func SetLogLevel(ctx context.Context, subsystem, level string) error {
	url := fmt.Sprintf("http://%s/log/level", viper.GetString("manage_api"))
	jsonStr, err := json.Marshal(map[string]string{"subsystem": subsystem, "level": level})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to set log level: %s", resp.Status)
	}

	return nil
}

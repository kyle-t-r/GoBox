package publib

import (
	"encoding/json"
	"os"
)

// ConfigFromStdin reads JSON configuration from stdin and unmarshals it into the provided interface
func ConfigFromStdin(v interface{}) error {
	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(v); err != nil {
		logger.Printf("ERROR: Failed to decode config from stdin: %v\n", err)
		return err
	}
	return nil
}

package mapper

import (
	"encoding/json"
	"fmt"
)

func TransformConfig[T any, D any](providerConfig D) (T, error) {
	var config T
	configBytes, err := json.Marshal(providerConfig)
	if err != nil {
		return config, fmt.Errorf("error marshaling provider config: %w", err)
	}

	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return config, fmt.Errorf("error unmarshaling to target config: %w", err)
	}

	return config, nil
}

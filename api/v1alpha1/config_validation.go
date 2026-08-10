package v1alpha1

import (
	"fmt"
	"strings"
)

const webhookNotificationSourceType = "Webhook"

// Validate checks cross-field constraints on ConfigSpec that cannot be expressed on individual fields.
func (s *ConfigSpec) Validate() error {
	if err := validateUniqueWebhookPathSuffixes(s.NotificationSources); err != nil {
		return err
	}
	return nil
}

func validateUniqueWebhookPathSuffixes(sources []NotificationSource) error {
	seen := map[string]int{}
	var duplicates []string

	for i, source := range sources {
		if source.Type != webhookNotificationSourceType {
			continue
		}

		suffix := ""
		if source.Webhook != nil {
			suffix = strings.TrimSpace(source.Webhook.PathSuffix)
		}

		if firstIndex, exists := seen[suffix]; exists {
			if suffix == "" {
				duplicates = append(duplicates, fmt.Sprintf(
					"notificationSources[%d] and notificationSources[%d] both omit webhook.pathSuffix; at most one webhook without pathSuffix is allowed",
					firstIndex, i,
				))
			} else {
				duplicates = append(duplicates, fmt.Sprintf(
					"notificationSources[%d].webhook.pathSuffix %q duplicates notificationSources[%d].webhook.pathSuffix",
					i, suffix, firstIndex,
				))
			}
			continue
		}
		seen[suffix] = i
	}

	if len(duplicates) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(duplicates, "; "))
}

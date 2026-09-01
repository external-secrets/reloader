// Package matchstrategy implements the Config CRD's optional MatchStrategy,
// which lets a destinationsToWatch entry decide for itself whether a given
// destination object is affected by an incoming SecretRotationEvent, instead
// of relying on the built-in per-destination-type References() check.
package matchstrategy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"k8s.io/client-go/util/jsonpath"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
)

// Evaluate reports whether obj is considered a match under ms, given the
// event currently being processed.
//
// ms.Path is evaluated against obj using Kubernetes JSONPath syntax (the
// same syntax `kubectl get -o jsonpath` uses), without the surrounding curly
// braces - e.g. "spec.dataFrom[*].extract.key". Every value found at that
// path is checked against every condition in ms.Conditions; obj matches if
// at least one found value satisfies all of the conditions.
//
// Each condition's Value is rendered as a Go template before comparison,
// with the event bound as the template's root, so conditions can reference
// data from the specific event being processed - for example:
//
//	conditions:
//	  - value: "{{ .SecretIdentifier }}"
//	    operation: ContainedBy
//
// Available template fields mirror events.SecretRotationEvent:
// .SecretIdentifier, .RotationTimestamp, .TriggerSource, .Namespace.
func Evaluate(ms *v1alpha1.MatchStrategy, obj client.Object, event events.SecretRotationEvent) (bool, error) {
	if ms == nil {
		return false, fmt.Errorf("matchStrategy must not be nil")
	}
	if ms.Path == "" {
		return false, fmt.Errorf("matchStrategy.path must not be empty")
	}
	if len(ms.Conditions) == 0 {
		return false, fmt.Errorf("matchStrategy.conditions must not be empty")
	}

	values, err := valuesAtPath(obj, ms.Path)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate matchStrategy.path %q: %w", ms.Path, err)
	}

	conditions := make([]renderedCondition, 0, len(ms.Conditions))
	for _, c := range ms.Conditions {
		rendered, err := renderConditionValue(c.Value, event)
		if err != nil {
			return false, fmt.Errorf("failed to render matchStrategy condition value %q: %w", c.Value, err)
		}
		conditions = append(conditions, renderedCondition{operation: c.Operation, value: rendered})
	}

	for _, v := range values {
		matched, err := allConditionsMatch(v, conditions)
		if err != nil {
			return false, fmt.Errorf("failed to evaluate matchStrategy.conditions against %q: %w", v, err)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

type renderedCondition struct {
	operation v1alpha1.ConditionOperation
	value     string
}

// allConditionsMatch reports whether fieldValue satisfies every condition.
// A malformed condition (e.g. an invalid regular expression, or an unknown
// operation) is a configuration error and is returned as such rather than
// silently treated as "no match", so it surfaces to whoever's watching the
// controller's logs instead of just quietly never firing.
func allConditionsMatch(fieldValue string, conditions []renderedCondition) (bool, error) {
	for _, c := range conditions {
		matched, err := evaluateCondition(fieldValue, c)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}
	return true, nil
}

func evaluateCondition(fieldValue string, c renderedCondition) (bool, error) {
	switch c.operation {
	case v1alpha1.ConditionOperationEqual:
		return fieldValue == c.value, nil
	case v1alpha1.ConditionOperationNotEqual:
		return fieldValue != c.value, nil
	case v1alpha1.ConditionOperationContains:
		return strings.Contains(fieldValue, c.value), nil
	case v1alpha1.ConditionOperationNotContains:
		return !strings.Contains(fieldValue, c.value), nil
	case v1alpha1.ConditionOperationContainedBy:
		return strings.Contains(c.value, fieldValue), nil
	case v1alpha1.ConditionOperationNotContainedBy:
		return !strings.Contains(c.value, fieldValue), nil
	case v1alpha1.ConditionOperationIn: // aka ConditionOperationRegularExpression
		re, err := regexp.Compile(c.value)
		if err != nil {
			return false, fmt.Errorf("invalid regular expression %q: %w", c.value, err)
		}
		return re.MatchString(fieldValue), nil
	default:
		return false, fmt.Errorf("unsupported condition operation %q", c.operation)
	}
}

// renderConditionValue renders a condition's Value field as a Go template,
// with event bound as the root of the template so authors can reference its
// exported fields directly (e.g. "{{ .SecretIdentifier }}"). Values with no
// template actions are returned unchanged.
func renderConditionValue(value string, event events.SecretRotationEvent) (string, error) {
	tpl, err := template.New("matchStrategyCondition").Option("missingkey=error").Parse(value)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, event); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// valuesAtPath evaluates a Kubernetes-style JSONPath expression (without the
// surrounding curly braces) against obj and returns every value found,
// stringified for comparison. A path that resolves to nothing (e.g. an
// unset optional field) yields an empty, non-error result.
func valuesAtPath(obj client.Object, path string) ([]string, error) {
	raw, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal object: %w", err)
	}
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal object: %w", err)
	}

	jp := jsonpath.New("matchStrategy").AllowMissingKeys(true)
	if err := jp.Parse(toJSONPathTemplate(path)); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	results, err := jp.FindResults(data)
	if err != nil {
		return nil, err
	}

	values := make([]string, 0)
	for _, set := range results {
		for _, v := range set {
			values = append(values, stringify(v))
		}
	}
	return values, nil
}

// toJSONPathTemplate turns a bare path such as "spec.dataFrom[*].extract.key"
// (the format used throughout this project's Config CRD and documentation)
// into the "{.spec.dataFrom[*].extract.key}" syntax client-go's
// jsonpath.Parse expects.
func toJSONPathTemplate(path string) string {
	return fmt.Sprintf("{.%s}", strings.TrimPrefix(path, "."))
}

func stringify(v reflect.Value) string {
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.String {
		return v.String()
	}
	if !v.IsValid() {
		return ""
	}
	return fmt.Sprintf("%v", v.Interface())
}

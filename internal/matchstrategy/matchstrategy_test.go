package matchstrategy

import (
	"reflect"
	"testing"

	esov1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
)

func externalSecretWithExtractKey(key string) *esov1.ExternalSecret {
	return &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ai-gateway", Namespace: "ai-gateway"},
		Spec: esov1.ExternalSecretSpec{
			DataFrom: []esov1.ExternalSecretDataFromRemoteRef{
				{Extract: &esov1.ExternalSecretDataRemoteRef{Key: key}},
			},
		},
	}
}

func externalSecretWithFindRegexp(re string) *esov1.ExternalSecret {
	return &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "default"},
		Spec: esov1.ExternalSecretSpec{
			DataFrom: []esov1.ExternalSecretDataFromRemoteRef{
				{Find: &esov1.ExternalSecretFind{Name: &esov1.FindName{RegExp: re}}},
			},
		},
	}
}

// TestEvaluate_ContainedBy_MatchesAWSARN verifies the motivating use case:
// an ExternalSecret referencing a secret by its AWS Secrets Manager friendly
// name is matched against an event whose SecretIdentifier is the full ARN
// (as delivered by the AwsSqs source), using ContainedBy plus templating.
func TestEvaluate_ContainedBy_MatchesAWSARN(t *testing.T) {
	es := externalSecretWithExtractKey("platform/ai-gateway/service-secrets")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "{{ .SecretIdentifier }}", Operation: v1alpha1.ConditionOperationContainedBy},
		},
	}
	event := events.SecretRotationEvent{
		SecretIdentifier: "arn:aws:secretsmanager:us-east-1:051826739313:secret:platform/ai-gateway/service-secrets-78YXTj",
	}

	matched, err := Evaluate(ms, es, event)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !matched {
		t.Error("expected ContainedBy match against the ARN's embedded friendly name")
	}
}

func TestEvaluate_ContainedBy_NoMatch(t *testing.T) {
	es := externalSecretWithExtractKey("platform/other-service/service-secrets")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "{{ .SecretIdentifier }}", Operation: v1alpha1.ConditionOperationContainedBy},
		},
	}
	event := events.SecretRotationEvent{
		SecretIdentifier: "arn:aws:secretsmanager:us-east-1:051826739313:secret:platform/ai-gateway/service-secrets-78YXTj",
	}

	matched, err := Evaluate(ms, es, event)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if matched {
		t.Error("expected no match for an unrelated secret key")
	}
}

func TestEvaluate_Equal(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "my-secret", Operation: v1alpha1.ConditionOperationEqual},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !matched {
		t.Error("expected exact match")
	}
}

func TestEvaluate_NotEqual(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "other-secret", Operation: v1alpha1.ConditionOperationNotEqual},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !matched {
		t.Error("expected NotEqual to match when values differ")
	}
}

func TestEvaluate_Contains(t *testing.T) {
	es := externalSecretWithExtractKey("platform/ai-gateway/service-secrets")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "ai-gateway", Operation: v1alpha1.ConditionOperationContains},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !matched {
		t.Error("expected Contains to match a substring of the field value")
	}
}

func TestEvaluate_NotContains(t *testing.T) {
	es := externalSecretWithExtractKey("platform/ai-gateway/service-secrets")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "jellyfish", Operation: v1alpha1.ConditionOperationNotContains},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !matched {
		t.Error("expected NotContains to match when the substring is absent")
	}
}

// TestEvaluate_RegularExpression mirrors the documented example: an
// ExternalSecret using dataFrom.find.name.regexp, matched via
// RegularExpression against a literal condition value.
func TestEvaluate_RegularExpression(t *testing.T) {
	es := externalSecretWithFindRegexp("prod/.*")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].find.name.regexp",
		Conditions: []v1alpha1.Condition{
			{Value: "prod/.*", Operation: v1alpha1.ConditionOperationIn},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !matched {
		t.Error("expected RegularExpression to match an identical pattern")
	}
}

// TestEvaluate_RegularExpression_InvalidPattern verifies that a malformed
// regular expression is surfaced as an error rather than silently treated
// as "no match" - a misconfigured MatchStrategy should be loud, not quietly
// inert.
func TestEvaluate_RegularExpression_InvalidPattern(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "(", Operation: v1alpha1.ConditionOperationIn},
		},
	}
	_, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err == nil {
		t.Fatal("expected an error when the regular expression fails to compile")
	}
}

// TestEvaluate_UnsupportedOperation verifies that an unrecognized operation
// string is surfaced as an error rather than silently treated as "no match".
func TestEvaluate_UnsupportedOperation(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "my-secret", Operation: v1alpha1.ConditionOperation("Equals")}, // typo: not "Equal"
		},
	}
	_, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err == nil {
		t.Fatal("expected an error for an unrecognized condition operation")
	}
}

func TestEvaluate_ContainedByInverse(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "totally-unrelated-arn-like-string", Operation: v1alpha1.ConditionOperationContainedBy},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if matched {
		t.Error("expected no match when the field value is not a substring of the condition value")
	}
}

func TestEvaluate_NotContainedBy(t *testing.T) {
	es := externalSecretWithExtractKey("platform/ai-gateway/service-secrets")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "totally-unrelated-arn-like-string", Operation: v1alpha1.ConditionOperationNotContainedBy},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !matched {
		t.Error("expected NotContainedBy to match when the field value is not embedded in the condition value")
	}
}

func TestEvaluate_NotContainedBy_NoMatchWhenContained(t *testing.T) {
	es := externalSecretWithExtractKey("platform/ai-gateway/service-secrets")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "{{ .SecretIdentifier }}", Operation: v1alpha1.ConditionOperationNotContainedBy},
		},
	}
	event := events.SecretRotationEvent{
		SecretIdentifier: "arn:aws:secretsmanager:us-east-1:051826739313:secret:platform/ai-gateway/service-secrets-78YXTj",
	}
	matched, err := Evaluate(ms, es, event)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if matched {
		t.Error("expected NotContainedBy to not match when the field value is embedded in the condition value")
	}
}

func TestEvaluate_MultipleConditionsAreANDed(t *testing.T) {
	es := externalSecretWithExtractKey("platform/ai-gateway/service-secrets")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "ai-gateway", Operation: v1alpha1.ConditionOperationContains},
			{Value: "jellyfish", Operation: v1alpha1.ConditionOperationContains},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if matched {
		t.Error("expected no match when not all conditions are satisfied")
	}
}

func TestEvaluate_MultipleValuesAtPathAreORed(t *testing.T) {
	es := &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "default"},
		Spec: esov1.ExternalSecretSpec{
			DataFrom: []esov1.ExternalSecretDataFromRemoteRef{
				{Extract: &esov1.ExternalSecretDataRemoteRef{Key: "other-secret"}},
				{Extract: &esov1.ExternalSecretDataRemoteRef{Key: "my-secret"}},
			},
		},
	}
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "my-secret", Operation: v1alpha1.ConditionOperationEqual},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !matched {
		t.Error("expected a match when any value found at path satisfies the conditions")
	}
}

func TestEvaluate_PathWithNoResultsDoesNotError(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.target.template.metadata.labels.does-not-exist",
		Conditions: []v1alpha1.Condition{
			{Value: "anything", Operation: v1alpha1.ConditionOperationEqual},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if matched {
		t.Error("expected no match when the path resolves to nothing")
	}
}

func TestEvaluate_RequiresPath(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	ms := &v1alpha1.MatchStrategy{
		Conditions: []v1alpha1.Condition{{Value: "x", Operation: v1alpha1.ConditionOperationEqual}},
	}
	if _, err := Evaluate(ms, es, events.SecretRotationEvent{}); err == nil {
		t.Error("expected an error when path is empty")
	}
}

func TestEvaluate_RequiresConditions(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	ms := &v1alpha1.MatchStrategy{Path: "spec.dataFrom[*].extract.key"}
	if _, err := Evaluate(ms, es, events.SecretRotationEvent{}); err == nil {
		t.Error("expected an error when conditions is empty")
	}
}

func TestEvaluate_RequiresNonNilStrategy(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	if _, err := Evaluate(nil, es, events.SecretRotationEvent{}); err == nil {
		t.Error("expected an error when matchStrategy is nil")
	}
}

func TestEvaluate_InvalidTemplateErrors(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "{{ .DoesNotExist }}", Operation: v1alpha1.ConditionOperationEqual},
		},
	}
	if _, err := Evaluate(ms, es, events.SecretRotationEvent{}); err == nil {
		t.Error("expected an error when the condition template references an unknown field")
	}
}

// TestEvaluate_MalformedTemplateSyntaxErrors covers a template that fails to
// even parse (as opposed to TestEvaluate_InvalidTemplateErrors, which fails
// at execution time referencing an unknown field).
func TestEvaluate_MalformedTemplateSyntaxErrors(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "{{ .SecretIdentifier ", Operation: v1alpha1.ConditionOperationEqual}, // unterminated action
		},
	}
	if _, err := Evaluate(ms, es, events.SecretRotationEvent{}); err == nil {
		t.Error("expected an error when the condition template fails to parse")
	}
}

// TestEvaluate_InvalidPathSyntaxErrors verifies that a malformed JSONPath
// expression is surfaced as an error, not silently treated as "no results".
func TestEvaluate_InvalidPathSyntaxErrors(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*", // unterminated bracket
		Conditions: []v1alpha1.Condition{
			{Value: "my-secret", Operation: v1alpha1.ConditionOperationEqual},
		},
	}
	if _, err := Evaluate(ms, es, events.SecretRotationEvent{}); err == nil {
		t.Error("expected an error for a malformed jsonpath expression")
	}
}

// TestEvaluate_AllTemplateFieldsAreAvailable exercises every documented
// template field (not just SecretIdentifier) to lock in the contract that
// all of events.SecretRotationEvent's exported fields are usable.
func TestEvaluate_AllTemplateFieldsAreAvailable(t *testing.T) {
	event := events.SecretRotationEvent{
		SecretIdentifier:  "secret-id",
		RotationTimestamp: "2026-07-23T17:25:37Z",
		TriggerSource:     "aws-secretsmanager",
		Namespace:         "ai-gateway",
	}

	tests := []struct {
		name     string
		template string
		key      string
	}{
		{"SecretIdentifier", "{{ .SecretIdentifier }}", "secret-id"},
		{"RotationTimestamp", "{{ .RotationTimestamp }}", "2026-07-23T17:25:37Z"},
		{"TriggerSource", "{{ .TriggerSource }}", "aws-secretsmanager"},
		{"Namespace", "{{ .Namespace }}", "ai-gateway"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := externalSecretWithExtractKey(tt.key)
			ms := &v1alpha1.MatchStrategy{
				Path: "spec.dataFrom[*].extract.key",
				Conditions: []v1alpha1.Condition{
					{Value: tt.template, Operation: v1alpha1.ConditionOperationEqual},
				},
			}
			matched, err := Evaluate(ms, es, event)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if !matched {
				t.Errorf("expected %s to render to %q and match", tt.template, tt.key)
			}
		})
	}
}

// TestEvaluate_MixedOperationsAreANDed verifies AND semantics across
// conditions using different operation types together, not just two
// conditions of the same operation.
func TestEvaluate_MixedOperationsAreANDed(t *testing.T) {
	es := externalSecretWithExtractKey("platform/ai-gateway/service-secrets")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "ai-gateway", Operation: v1alpha1.ConditionOperationContains},
			{Value: "platform/ai-gateway/service-secrets", Operation: v1alpha1.ConditionOperationEqual},
			{Value: "jellyfish", Operation: v1alpha1.ConditionOperationNotContains},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !matched {
		t.Error("expected a match when every condition, of different operation types, is individually satisfied")
	}
}

// TestEvaluate_ORAndANDCombine verifies that when path resolves to multiple
// values, and only some of those values satisfy the full set of ANDed
// conditions, the overall result is still a match (OR-across-values,
// AND-across-conditions must compose correctly, not just work in isolation).
func TestEvaluate_ORAndANDCombine(t *testing.T) {
	es := &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "default"},
		Spec: esov1.ExternalSecretSpec{
			DataFrom: []esov1.ExternalSecretDataFromRemoteRef{
				// Satisfies Contains("ai-gateway") but not Contains("jellyfish").
				{Extract: &esov1.ExternalSecretDataRemoteRef{Key: "platform/ai-gateway/service-secrets"}},
				// Satisfies neither condition.
				{Extract: &esov1.ExternalSecretDataRemoteRef{Key: "unrelated"}},
				// Satisfies both conditions - this is the one that should make it match overall.
				{Extract: &esov1.ExternalSecretDataRemoteRef{Key: "platform/jellyfish-ai-gateway/service-secrets"}},
			},
		},
	}
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "ai-gateway", Operation: v1alpha1.ConditionOperationContains},
			{Value: "jellyfish", Operation: v1alpha1.ConditionOperationContains},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !matched {
		t.Error("expected a match because at least one of the values at path satisfies every condition")
	}
}

// TestEvaluate_EmptyConditionValueContainsEverything documents (deliberately,
// not as a desired feature) that ContainedBy/Contains with an empty rendered
// condition value behaves like strings.Contains(s, "") always does: it
// matches unconditionally. This is exercised explicitly so the behavior is
// locked in and visible, since it's easy to hit by accident (e.g.
// templating an event field that happens to be empty).
func TestEvaluate_EmptyConditionValueContainsEverything(t *testing.T) {
	es := externalSecretWithExtractKey("platform/ai-gateway/service-secrets")
	ms := &v1alpha1.MatchStrategy{
		Path: "spec.dataFrom[*].extract.key",
		Conditions: []v1alpha1.Condition{
			{Value: "{{ .Namespace }}", Operation: v1alpha1.ConditionOperationContains}, // Namespace left unset on the event
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !matched {
		t.Error("expected Contains against an empty string to match, per strings.Contains semantics")
	}
}

// TestEvaluate_NonStringFieldValue verifies that a path resolving to a
// non-string JSON value (a number here) is stringified and compared
// correctly rather than causing an error or a silent non-match.
func TestEvaluate_NonStringFieldValue(t *testing.T) {
	es := &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "default", Generation: 5},
		Spec:       esov1.ExternalSecretSpec{},
	}
	ms := &v1alpha1.MatchStrategy{
		Path: "metadata.generation",
		Conditions: []v1alpha1.Condition{
			{Value: "5", Operation: v1alpha1.ConditionOperationEqual},
		},
	}
	matched, err := Evaluate(ms, es, events.SecretRotationEvent{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !matched {
		t.Error("expected a numeric field value to stringify to a plain integer for comparison")
	}
}

// --- White-box tests for unexported helpers ---

func TestToJSONPathTemplate(t *testing.T) {
	tests := map[string]string{
		"spec.dataFrom[*].extract.key": "{.spec.dataFrom[*].extract.key}",
		".spec.dataFrom.key":           "{.spec.dataFrom.key}",
		"metadata.name":                "{.metadata.name}",
	}
	for in, want := range tests {
		if got := toJSONPathTemplate(in); got != want {
			t.Errorf("toJSONPathTemplate(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStringify(t *testing.T) {
	strPtr := "hello"
	var nilStrPtr *string
	var nilAnyPtr any

	tests := []struct {
		name string
		val  reflect.Value
		want string
	}{
		{"string", reflect.ValueOf("hello"), "hello"},
		{"empty string", reflect.ValueOf(""), ""},
		{"bool true", reflect.ValueOf(true), "true"},
		{"bool false", reflect.ValueOf(false), "false"},
		{"float64 whole number", reflect.ValueOf(float64(5)), "5"},
		{"float64 fractional", reflect.ValueOf(float64(1.5)), "1.5"},
		{"pointer to string", reflect.ValueOf(&strPtr), "hello"},
		// A nil *string, wrapped in an addressable interface so stringify sees
		// a Ptr kind to dereference (this is the shape jsonpath.FindResults
		// actually returns for an unset pointer field), must not panic.
		{"nil pointer", reflect.ValueOf(&nilStrPtr).Elem(), ""},
		// reflect.ValueOf(nil) (a truly untyped nil interface) is the
		// zero reflect.Value and must not panic either.
		{"invalid/zero value", reflect.ValueOf(nilAnyPtr), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringify(tt.val); got != tt.want {
				t.Errorf("stringify(...) = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestEvaluate_PathRuntimeErrorErrors covers a path that parses successfully
// but fails at evaluation time - e.g. indexing into a field that isn't a
// list - as distinct from TestEvaluate_InvalidPathSyntaxErrors, which fails
// to parse at all.
func TestEvaluate_PathRuntimeErrorErrors(t *testing.T) {
	es := externalSecretWithExtractKey("my-secret")
	ms := &v1alpha1.MatchStrategy{
		Path: "metadata.name[0]", // metadata.name is a string, not a list
		Conditions: []v1alpha1.Condition{
			{Value: "m", Operation: v1alpha1.ConditionOperationEqual},
		},
	}
	if _, err := Evaluate(ms, es, events.SecretRotationEvent{}); err == nil {
		t.Error("expected an error when the path indexes into a non-list field")
	}
}

// unmarshalableObject is a minimal client.Object whose JSON marshaling
// always fails (complex128 has no JSON representation), used to exercise
// valuesAtPath's json.Marshal error branch, which no real Kubernetes object
// can trigger.
type unmarshalableObject struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Bad complex128 `json:"bad"`
}

func (u *unmarshalableObject) DeepCopyObject() runtime.Object {
	cp := *u
	return &cp
}

func TestEvaluate_ObjectMarshalErrorErrors(t *testing.T) {
	obj := &unmarshalableObject{Bad: complex(1, 2)}
	ms := &v1alpha1.MatchStrategy{
		Path: "bad",
		Conditions: []v1alpha1.Condition{
			{Value: "anything", Operation: v1alpha1.ConditionOperationEqual},
		},
	}
	if _, err := Evaluate(ms, obj, events.SecretRotationEvent{}); err == nil {
		t.Error("expected an error when the destination object cannot be marshaled to JSON")
	}
}

func TestRenderConditionValue(t *testing.T) {
	event := events.SecretRotationEvent{SecretIdentifier: "my-id"}

	got, err := renderConditionValue("plain-string-no-template", event)
	if err != nil {
		t.Fatalf("renderConditionValue: %v", err)
	}
	if got != "plain-string-no-template" {
		t.Errorf("expected a literal value with no template actions to be returned unchanged, got %q", got)
	}

	got, err = renderConditionValue("id={{ .SecretIdentifier }}", event)
	if err != nil {
		t.Fatalf("renderConditionValue: %v", err)
	}
	if got != "id=my-id" {
		t.Errorf("expected template interpolation, got %q", got)
	}
}

package v1alpha1

type MatchStrategy struct {
	Path       string      `json:"path"`
	Conditions []Condition `json:"conditions"`
}

type Condition struct {
	Value     string             `json:"value"`
	Operation ConditionOperation `json:"operation"`
}

type ConditionOperation string

const (
	ConditionOperationEqual       ConditionOperation = "Equal"
	ConditionOperationNotEqual    ConditionOperation = "NotEqual"
	ConditionOperationContains    ConditionOperation = "Contains"
	ConditionOperationNotContains ConditionOperation = "NotContains"
	// ConditionOperationIn matches when the value found at Path matches the
	// regular expression given in Value. Named "In" for historical reasons;
	// ConditionOperationRegularExpression is provided as a clearer alias.
	ConditionOperationIn ConditionOperation = "RegularExpression"
	// ConditionOperationContainedBy matches when the value found at Path is a
	// substring of Value - the inverse direction of Contains. This is what
	// lets a MatchStrategy correlate a destination's own configured key
	// (e.g. an ExternalSecret's spec.dataFrom[].extract.key) against a
	// broader identifier surfaced by a notification source (e.g. an AWS
	// Secrets Manager ARN), where the destination's key is a substring of,
	// rather than equal to, the identifier on the incoming event.
	ConditionOperationContainedBy ConditionOperation = "ContainedBy"
	// ConditionOperationNotContainedBy is the negation of ConditionOperationContainedBy.
	ConditionOperationNotContainedBy ConditionOperation = "NotContainedBy"
)

// ConditionOperationRegularExpression is a clearer alias for ConditionOperationIn.
const ConditionOperationRegularExpression = ConditionOperationIn

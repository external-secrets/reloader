package util

import "regexp"

// awsSecretsManagerARNPattern matches AWS Secrets Manager secret ARNs, e.g.
//
//	arn:aws:secretsmanager:us-east-1:123456789012:secret:platform/ai-gateway/service-secrets-78YXTj
//
// Secrets Manager always appends a hyphen followed by a random 6-character
// alphanumeric suffix to a secret's friendly name when it generates the ARN
// for that secret - see:
// https://docs.aws.amazon.com/secretsmanager/latest/userguide/troubleshoot.html#ARN_secretnamehyphen
//
// Group 1 captures the friendly name (everything between "secret:" and the
// AWS-appended suffix). The partition segment allows for aws, aws-cn and
// aws-us-gov ARNs.
var awsSecretsManagerARNPattern = regexp.MustCompile(`^arn:aws[a-zA-Z0-9-]*:secretsmanager:[^:]*:[^:]*:secret:(.+)-[A-Za-z0-9]{6}$`)

// SecretIdentifierAliases returns every identifier form that should be
// considered equivalent to identifier when checking whether a destination
// resource (ExternalSecret, PushSecret, ...) references the secret an event
// is about.
//
// Notification sources report the identifier exactly as the upstream
// provider's API surfaced it. For AWS Secrets Manager, sources that derive
// the identifier from CloudTrail request parameters (e.g. the AwsSqs source
// reading requestParameters.secretId from a PutSecretValue/RotateSecret
// event) get the full secret ARN, including the random suffix AWS appends -
// not the friendly name. Both manual PutSecretValue calls made via the AWS
// console/CLI with the ARN as --secret-id and Secrets Manager's own built-in
// rotation Lambdas invoke PutSecretValue with the ARN as SecretId, so this is
// the common case, not an edge case.
//
// Meanwhile, ExternalSecret/PushSecret specs conventionally reference
// secrets by friendly name (e.g. spec.dataFrom[].extract.key:
// "platform/ai-gateway/service-secrets"), since that's what most SecretStore
// provider configs expect and what ties the manifest to a stable value
// across secret recreation. Comparing the two forms with strict equality
// means the reference check never succeeds for AWS-sourced events, so
// reload destinations are silently skipped even though the pipeline
// (listener, auth, event delivery) is working correctly.
func SecretIdentifierAliases(identifier string) []string {
	aliases := []string{identifier}
	if m := awsSecretsManagerARNPattern.FindStringSubmatch(identifier); m != nil {
		aliases = append(aliases, m[1])
	}
	return aliases
}

// SecretIdentifierMatches reports whether candidate - a value taken from a
// destination resource's spec, such as an ExternalSecret's
// spec.dataFrom[].extract.key or a PushSecret's spec.data[].match.remoteRef.remoteKey -
// refers to the same secret as identifier, the raw identifier carried on a
// SecretRotationEvent.
func SecretIdentifierMatches(candidate, identifier string) bool {
	for _, alias := range SecretIdentifierAliases(identifier) {
		if candidate == alias {
			return true
		}
	}
	return false
}

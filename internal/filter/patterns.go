// Package filter provides sensitive data filtering for Terraform state files.
package filter

// DefaultOmitResourceTypes are resource types that should be omitted entirely
// from state uploads because they inherently contain sensitive data.
var DefaultOmitResourceTypes = []string{
	// AWS Secrets
	"aws_secretsmanager_secret_version",
	"aws_ssm_parameter", // Often contains SecureString values

	// Random/Generated secrets
	"random_password",
	"random_string", // When used for secrets

	// TLS/Certificates
	"tls_private_key",
	"acme_certificate",
	"tls_self_signed_cert",
	"tls_locally_signed_cert",

	// Vault secrets
	"vault_generic_secret",
	"vault_kv_secret",
	"vault_kv_secret_v2",

	// Azure secrets
	"azurerm_key_vault_secret",
	"azurerm_key_vault_key",
	"azurerm_key_vault_certificate",

	// Google secrets
	"google_secret_manager_secret_version",
}

// DefaultOmitAttributes are attribute name patterns that should be omitted
// from resource attributes when found.
var DefaultOmitAttributes = []string{
	// Passwords
	"password",
	"master_password",
	"admin_password",
	"root_password",
	"db_password",

	// Secrets and tokens
	"secret",
	"secret_string",
	"secret_binary",
	"api_key",
	"api_secret",
	"token",
	"auth_token",
	"access_token",
	"refresh_token",

	// Keys
	"private_key",
	"private_key_pem",
	"private_key_openssh",
	"ssh_private_key",
	"access_key",
	"secret_key",
	"secret_access_key",

	// Credentials
	"credential",
	"credentials",
	"connection_string",
	"connection_url",

	// Certificates (private parts)
	"certificate_pem",
	"certificate_chain",
	"issuer_pem",

	// Other sensitive values
	"sensitive_value",
	"encrypted_value",
}

// AttributeContainsPattern checks if an attribute name contains any of the given patterns.
// The match is case-insensitive and checks for substring matches.
func AttributeContainsPattern(attrName string, patterns []string) bool {
	_, found := AttributeMatchingPattern(attrName, patterns)
	return found
}

// AttributeMatchingPattern checks if an attribute name contains any of the given patterns
// and returns the matching pattern if found.
func AttributeMatchingPattern(attrName string, patterns []string) (string, bool) {
	lowerAttr := toLowerCase(attrName)
	for _, pattern := range patterns {
		if containsIgnoreCase(lowerAttr, pattern) {
			return pattern, true
		}
	}
	return "", false
}

// ResourceTypeMatches checks if a resource type matches any of the given types.
func ResourceTypeMatches(resourceType string, types []string) bool {
	for _, t := range types {
		if resourceType == t {
			return true
		}
	}
	return false
}

// toLowerCase converts a string to lowercase (simple ASCII version).
func toLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// containsIgnoreCase checks if s contains substr (both already lowercase).
func containsIgnoreCase(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

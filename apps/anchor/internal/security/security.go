package security

import (
	"context"
	"crypto/rand"
	"fmt"
	"hash/crc32"
	"math/big"
	"strings"

	"github.com/nanostack-dev/shared/toolkit"
)

const (
	//nolint:gosec //only prefix
	DefaultProductAPIKeyPrefix = "anchor_prd_apikey_"
	//nolint:gosec //only prefix
	DefaultOrganizationAPIKeyPrefix = "anchor_org_apikey_"
	httpStatusForbidden             = 403
	apiKeyByteLength                = 48
	minKeyLength                    = 12
	crc32ChecksumLength             = 8
	visibleSuffixLength             = 4
	obfuscatePrefixLength           = 4
	obfuscateSuffixLength           = 4
	charset                         = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

type contextKey string

const (
	currentUserIDKey contextKey = "current_user_id"
	tenantIDKey      contextKey = "tenant_id"
)

// GetCurrentUserID safely retrieves the current user Name from context.
func GetCurrentUserID(ctx context.Context) (string, error) {
	value := ctx.Value(currentUserIDKey)
	if value == nil {
		return "", toolkit.NewNanostackErrorsWithStatus(
			"UNAUTHORIZED_ACCESS",
			"User authentication required",
			httpStatusForbidden,
		)
	}
	userID, ok := value.(string)
	if !ok {
		return "", toolkit.NewNanostackErrorsWithStatus(
			"INVALID_USER_CONTEXT",
			"Invalid user context data",
			httpStatusForbidden,
		)
	}
	return userID, nil
}

// GetTenantID safely retrieves the tenant Name from context.
func GetTenantID(ctx context.Context) (string, error) {
	value := ctx.Value(tenantIDKey)
	if value == nil {
		return "", toolkit.NewNanostackErrorsWithStatus(
			"UNAUTHORIZED_ACCESS",
			"Tenant authentication required",
			httpStatusForbidden,
		)
	}
	tenantID, ok := value.(string)
	if !ok {
		return "", toolkit.NewNanostackErrorsWithStatus(
			"INVALID_TENANT_CONTEXT",
			"Invalid tenant context data",
			httpStatusForbidden,
		)
	}
	return tenantID, nil
}

func SetCurrentUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, currentUserIDKey, userID)
}

func SetTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}
func IsValidProductAPIKey(apiKey string) bool {
	return validateSecretFormat(DefaultProductAPIKeyPrefix, apiKey)
}

func IsValidOrganizationAPIKey(apiKey string) bool {
	return validateSecretFormat(DefaultOrganizationAPIKeyPrefix, apiKey)
}

func GenerateProductAPIKey() (string, error) {
	return generateSecret(DefaultProductAPIKeyPrefix)
}

func GenerateOrganizationAPIKey() (string, error) {
	return generateSecret(DefaultOrganizationAPIKeyPrefix)
}

func ProductAPIKeyLength() int {
	return apiKeyLength(DefaultProductAPIKeyPrefix)
}

func OrganizationAPIKeyLength() int {
	return apiKeyLength(DefaultOrganizationAPIKeyPrefix)
}

// generateSecret generates secret for anchor, it uses a prefix, random bytes, and a CRC32 checksum.
func generateSecret(prefix string) (string, error) {
	bytes := make([]byte, apiKeyByteLength)
	for i := range bytes {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", toolkit.ErrUnexpected
		}
		bytes[i] = charset[n.Int64()]
	}
	checksum := crc32.Checksum(bytes, crc32.MakeTable(crc32.IEEE))
	hexChecksum := fmt.Sprintf("%08x", checksum)
	return prefix + string(bytes) + "_" + hexChecksum, nil
}

func HashSecret(apiKey string) string {
	return toolkit.HashSHA256String(apiKey)
}

func apiKeyLength(prefix string) int {
	// The length of the API key is:
	// - prefix length
	// - apiKeyByteLength (random bytes)
	// - crc32ChecksumLength (checksum)
	// - 1 (underscore separator)
	return len(prefix) + apiKeyByteLength + crc32ChecksumLength + 1
}

func validateSecretFormat(prefix, apiKey string) bool {
	switch prefix {
	case DefaultProductAPIKeyPrefix, DefaultOrganizationAPIKeyPrefix:
		if len(apiKey) < apiKeyLength(prefix) {
			return false
		}
	default:
		return false
	}
	if !strings.HasPrefix(apiKey, prefix) {
		return false
	}
	crc32checksum := apiKey[len(apiKey)-crc32ChecksumLength:]
	value := apiKey[len(prefix) : len(apiKey)-crc32ChecksumLength-1]
	checksum := crc32.Checksum([]byte(value), crc32.MakeTable(crc32.IEEE))
	expectedChecksum := fmt.Sprintf("%08x", checksum)
	return strings.EqualFold(crc32checksum, expectedChecksum)
}

func CompareAPIKey(apiKey, hashedAPIKey string) bool {
	return toolkit.CompareSHA256Hash(apiKey, hashedAPIKey)
}

func ObfuscateProductAPIKey(apiKey string) string {
	prefix := DefaultProductAPIKeyPrefix
	crc32Part := apiKey[len(apiKey)-crc32ChecksumLength-1:]
	return prefix + "***" + crc32Part
}

func ObfuscateOrganizationAPIKey(apiKey string) string {
	prefix := DefaultOrganizationAPIKeyPrefix
	crc32Part := apiKey[len(apiKey)-crc32ChecksumLength-1:]
	return prefix + "***" + crc32Part
}

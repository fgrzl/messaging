package natsbroker

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetDefaultOptions(t *testing.T) {
	t.Run("ShouldReturnDefaultOptionsFromEnvironment", func(t *testing.T) {
		// Arrange - set up test environment variables
		testCases := map[string]string{
			envHost:             "test-host",
			envWebSocketPort:    "9223",
			envMonitorPort:      "8223",
			envCertFile:         "/path/to/cert.pem",
			envKeyFile:          "/path/to/key.pem",
			envOperatorJWT:      "test-operator-jwt",
			envAccountJWT:       "test-account-jwt",
			envReadinessTimeout: "10s",
			envShutdownTimeout:  "15s",
		}

		// Set environment variables
		for key, value := range testCases {
			os.Setenv(key, value)
		}

		// Clean up after test
		defer func() {
			for key := range testCases {
				os.Unsetenv(key)
			}
		}()

		// Act
		opts := GetDefaultOptions()

		// Assert
		assert.Equal(t, "test-host", opts.Host)
		assert.Equal(t, 9223, opts.WebSocketPort)
		assert.Equal(t, 8223, opts.MonitorPort)
		assert.Equal(t, "/path/to/cert.pem", opts.CertFile)
		assert.Equal(t, "/path/to/key.pem", opts.KeyFile)
		assert.True(t, opts.EnableTLS) // Should be true when both cert and key are set
		assert.Equal(t, "test-operator-jwt", opts.OperatorJWT)
		assert.Equal(t, "test-account-jwt", opts.AccountJWT)
		assert.Equal(t, 10*time.Second, opts.ReadinessTimeout)
		assert.Equal(t, 15*time.Second, opts.ShutdownTimeout)
	})

	t.Run("ShouldUseDefaultValuesWhenEnvironmentNotSet", func(t *testing.T) {
		// Arrange - ensure environment variables are not set
		envVars := []string{
			envHost, envWebSocketPort, envMonitorPort, envCertFile,
			envKeyFile, envOperatorJWT, envAccountJWT, envReadinessTimeout, envShutdownTimeout,
		}

		for _, envVar := range envVars {
			os.Unsetenv(envVar)
		}

		// Act
		opts := GetDefaultOptions()

		// Assert
		assert.Equal(t, "localhost", opts.Host)
		assert.Equal(t, 9222, opts.WebSocketPort)
		assert.Equal(t, 8222, opts.MonitorPort)
		assert.Empty(t, opts.CertFile)
		assert.Empty(t, opts.KeyFile)
		assert.False(t, opts.EnableTLS) // Should be false when cert/key not set
		assert.Empty(t, opts.OperatorJWT)
		assert.Empty(t, opts.AccountJWT)
		assert.Equal(t, 5*time.Second, opts.ReadinessTimeout)
		assert.Equal(t, 10*time.Second, opts.ShutdownTimeout)
	})

	t.Run("ShouldHandleInvalidDurationFormats", func(t *testing.T) {
		// Arrange
		os.Setenv(envReadinessTimeout, "invalid-duration")
		os.Setenv(envShutdownTimeout, "not-a-duration")

		defer func() {
			os.Unsetenv(envReadinessTimeout)
			os.Unsetenv(envShutdownTimeout)
		}()

		// Act
		opts := GetDefaultOptions()

		// Assert - should fallback to defaults when parsing fails
		assert.Equal(t, 5*time.Second, opts.ReadinessTimeout)
		assert.Equal(t, 10*time.Second, opts.ShutdownTimeout)
	})

	t.Run("ShouldHandleInvalidPortNumbers", func(t *testing.T) {
		// Arrange
		os.Setenv(envWebSocketPort, "not-a-number")
		os.Setenv(envMonitorPort, "invalid-port")

		defer func() {
			os.Unsetenv(envWebSocketPort)
			os.Unsetenv(envMonitorPort)
		}()

		// Act
		opts := GetDefaultOptions()

		// Assert - should fallback to defaults when parsing fails
		assert.Equal(t, 9222, opts.WebSocketPort)
		assert.Equal(t, 8222, opts.MonitorPort)
	})
}

func TestMustGetDefaultOptions(t *testing.T) {
	t.Run("ShouldReturnOptionsWhenValid", func(t *testing.T) {
		// Arrange - set up valid configuration
		os.Setenv(envOperatorJWT, "valid-operator-jwt")
		os.Setenv(envAccountJWT, "valid-account-jwt")

		defer func() {
			os.Unsetenv(envOperatorJWT)
			os.Unsetenv(envAccountJWT)
		}()

		// Act & Assert - should not panic
		opts := MustGetDefaultOptions()
		assert.NotEmpty(t, opts.OperatorJWT)
		assert.NotEmpty(t, opts.AccountJWT)
	})

	t.Run("ShouldPanicWhenOptionsAreInvalid", func(t *testing.T) {
		// Arrange - clear required environment variables
		os.Unsetenv(envOperatorJWT)
		os.Unsetenv(envOperatorJWTURL)
		os.Unsetenv(envAccountJWT)
		os.Unsetenv(envAccountJWTURL)

		// Act & Assert
		assert.Panics(t, func() {
			MustGetDefaultOptions()
		})
	})
}

func TestBrokerOptions_Validate(t *testing.T) {
	tests := []struct {
		name        string
		options     BrokerOptions
		expectError bool
		errorMsg    string
	}{
		{
			name: "ShouldPassValidationWithOperatorJWTAndAccountJWT",
			options: BrokerOptions{
				OperatorJWT: "valid-operator-jwt",
				AccountJWT:  "valid-account-jwt",
			},
			expectError: false,
		},
		{
			name: "ShouldPassValidationWithJWTURLs",
			options: BrokerOptions{
				OperatorJWTURL: "https://example.com/operator.jwt",
				AccountJWTURL:  "https://example.com/account.jwt",
			},
			expectError: false,
		},
		{
			name: "ShouldPassValidationWithMixedJWTSources",
			options: BrokerOptions{
				OperatorJWT:   "valid-operator-jwt",
				AccountJWTURL: "https://example.com/account.jwt",
			},
			expectError: false,
		},
		{
			name: "ShouldFailValidationWhenTLSEnabledButCertMissing",
			options: BrokerOptions{
				EnableTLS:   true,
				KeyFile:     "/path/to/key.pem",
				OperatorJWT: "valid-operator-jwt",
				AccountJWT:  "valid-account-jwt",
			},
			expectError: true,
			errorMsg:    "EnableTLS is true but cert or key file is missing",
		},
		{
			name: "ShouldFailValidationWhenTLSEnabledButKeyMissing",
			options: BrokerOptions{
				EnableTLS:   true,
				CertFile:    "/path/to/cert.pem",
				OperatorJWT: "valid-operator-jwt",
				AccountJWT:  "valid-account-jwt",
			},
			expectError: true,
			errorMsg:    "EnableTLS is true but cert or key file is missing",
		},
		{
			name: "ShouldFailValidationWhenOperatorJWTMissing",
			options: BrokerOptions{
				AccountJWT: "valid-account-jwt",
			},
			expectError: true,
			errorMsg:    "either OperatorJWT or OperatorJWTURL must be provided",
		},
		{
			name: "ShouldFailValidationWhenAccountJWTMissing",
			options: BrokerOptions{
				OperatorJWT: "valid-operator-jwt",
			},
			expectError: true,
			errorMsg:    "either AccountJWT or AccountJWTURL must be provided",
		},
		{
			name:        "ShouldFailValidationWhenAllJWTSourcesMissing",
			options:     BrokerOptions{},
			expectError: true,
			errorMsg:    "either OperatorJWT or OperatorJWTURL must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := tt.options.Validate()

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	t.Run("ShouldReturnEnvironmentValueWhenSet", func(t *testing.T) {
		// Arrange
		key := "TEST_ENV_VAR"
		value := "test-value"
		fallback := "fallback-value"
		os.Setenv(key, value)
		defer os.Unsetenv(key)

		// Act
		result := getEnvOrDefault(key, fallback)

		// Assert
		assert.Equal(t, value, result)
	})

	t.Run("ShouldReturnFallbackWhenEnvironmentNotSet", func(t *testing.T) {
		// Arrange
		key := "NON_EXISTENT_ENV_VAR"
		fallback := "fallback-value"
		os.Unsetenv(key) // Ensure it's not set

		// Act
		result := getEnvOrDefault(key, fallback)

		// Assert
		assert.Equal(t, fallback, result)
	})

	t.Run("ShouldReturnFallbackWhenEnvironmentIsEmpty", func(t *testing.T) {
		// Arrange
		key := "EMPTY_ENV_VAR"
		fallback := "fallback-value"
		os.Setenv(key, "")
		defer os.Unsetenv(key)

		// Act
		result := getEnvOrDefault(key, fallback)

		// Assert
		assert.Equal(t, fallback, result)
	})
}

func TestGetEnvOrDefaultInt(t *testing.T) {
	t.Run("ShouldReturnParsedIntegerWhenValid", func(t *testing.T) {
		// Arrange
		key := "TEST_INT_VAR"
		value := "12345"
		fallback := 9999
		os.Setenv(key, value)
		defer os.Unsetenv(key)

		// Act
		result := getEnvOrDefaultInt(key, fallback)

		// Assert
		assert.Equal(t, 12345, result)
	})

	t.Run("ShouldReturnFallbackWhenEnvironmentNotSet", func(t *testing.T) {
		// Arrange
		key := "NON_EXISTENT_INT_VAR"
		fallback := 9999
		os.Unsetenv(key)

		// Act
		result := getEnvOrDefaultInt(key, fallback)

		// Assert
		assert.Equal(t, fallback, result)
	})

	t.Run("ShouldReturnFallbackWhenEnvironmentIsInvalid", func(t *testing.T) {
		// Arrange
		key := "INVALID_INT_VAR"
		value := "not-a-number"
		fallback := 9999
		os.Setenv(key, value)
		defer os.Unsetenv(key)

		// Act
		result := getEnvOrDefaultInt(key, fallback)

		// Assert
		assert.Equal(t, fallback, result)
	})

	t.Run("ShouldReturnFallbackWhenEnvironmentIsEmpty", func(t *testing.T) {
		// Arrange
		key := "EMPTY_INT_VAR"
		fallback := 9999
		os.Setenv(key, "")
		defer os.Unsetenv(key)

		// Act
		result := getEnvOrDefaultInt(key, fallback)

		// Assert
		assert.Equal(t, fallback, result)
	})
}

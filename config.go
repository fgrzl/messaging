package messaging

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Define environment variables as constants
const (
	FGRZL_TLS_CERT_PATH  = "FGRZL_TLS_CERT_PATH"
	FGRZL_TLS_KEY_PATH   = "FGRZL_TLS_KEY_PATH"
	FGRZL_CLIENT_POOLS   = "FGRZL_CLIENT_POOLS"
	FGRZL_BROKER_PORT    = "FGRZL_BROKER_PORT"
	FGRZL_BROKER_USE_TLS = "FGRZL_BROKER_USE_TLS"
	FGRZL_WEB_PORT       = "FGRZL_WEB_PORT"
	FGRZL_WEB_USE_TLS    = "FGRZL_WEB_USE_TLS"
)

// GetCertFile returns the value of the FGRZL_TLS_CERT_PATH environment variable
func GetCertFilePath() string {
	return os.Getenv(FGRZL_TLS_CERT_PATH)
}

// SetCertFile sets the value of the FGRZL_TLS_CERT_PATH environment variable
func SetCertFilePath(value string) error {
	return os.Setenv(FGRZL_TLS_CERT_PATH, value)
}

// GetKeyFile returns the value of the FGRZL_TLS_KEY_PATH environment variable
func GetKeyFilePath() string {
	return os.Getenv(FGRZL_TLS_KEY_PATH)
}

// SetKeyFile sets the value of the FGRZL_TLS_KEY_PATH environment variable
func SetKeyFilePath(value string) error {
	return os.Setenv(FGRZL_TLS_KEY_PATH, value)
}

func GetClientPools() []string {
	return strings.Split(os.Getenv(FGRZL_CLIENT_POOLS), ",")
}

func SetClientPools(values ...string) error {
	return os.Setenv(FGRZL_CLIENT_POOLS, strings.Join(values, ","))
}

func GetBrokerConnection() string {

	// ws://localhost:8080 or wss://localhost:8080
	scheme := "ws"
	host := "localhost"
	port := GetBrokerPort()
	return fmt.Sprintf("%s://%s:%d", scheme, host, port)
}

func GetBrokerPort() int {
	port, err := strconv.Atoi(os.Getenv(FGRZL_BROKER_PORT))
	if err != nil {
		return 0
	}
	return port
}

func SetBrokerPort(value int) error {
	return os.Setenv(FGRZL_BROKER_PORT, strconv.Itoa(value))
}

func GetBrokerUseTLS() bool {
	return os.Getenv(FGRZL_BROKER_USE_TLS) == "true"
}

func SetBrokerUseTLS(value bool) error {
	return os.Setenv(FGRZL_BROKER_USE_TLS, strconv.FormatBool(value))
}

func GetWebPort() int {
	port, err := strconv.Atoi(os.Getenv(FGRZL_WEB_PORT))
	if err != nil {
		return 0
	}
	return port
}

func SetWebPort(value int) error {
	return os.Setenv(FGRZL_WEB_PORT, strconv.Itoa(value))
}

func GetWebUseTLS() bool {
	return os.Getenv(FGRZL_WEB_USE_TLS) == "true"
}

func SetWebUseTLS(value bool) error {
	return os.Setenv(FGRZL_WEB_USE_TLS, strconv.FormatBool(value))
}

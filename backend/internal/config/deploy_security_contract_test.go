package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func deploySecurityRepoRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
}

func readDeploySecurityFile(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(deploySecurityRepoRoot(t), "deploy", name))
	require.NoError(t, err)
	return string(content)
}

func TestDeployConfigExampleUsesSafeBootstrapDefaults(t *testing.T) {
	var example struct {
		Server struct {
			Host           string   `yaml:"host"`
			TrustedProxies []string `yaml:"trusted_proxies"`
		} `yaml:"server"`
		JWT struct {
			Secret string `yaml:"secret"`
		} `yaml:"jwt"`
		Default struct {
			AdminPassword string `yaml:"admin_password"`
		} `yaml:"default"`
	}

	err := yaml.Unmarshal([]byte(readDeploySecurityFile(t, "config.example.yaml")), &example)
	require.NoError(t, err, "the example config must not contain duplicate YAML keys")
	require.Empty(t, example.JWT.Secret, "bootstrap should generate and persist the JWT secret")
	require.Empty(t, example.Default.AdminPassword, "the legacy default must not advertise a weak password")
	require.Equal(t, "127.0.0.1", example.Server.Host)
	require.Equal(t, []string{"127.0.0.1/32", "::1/128"}, example.Server.TrustedProxies)
}

func TestComposeDefaultsBindApplicationToLocalhost(t *testing.T) {
	for _, name := range []string{"docker-compose.yml", "docker-compose.standalone.yml"} {
		content := readDeploySecurityFile(t, name)
		require.Contains(t, content, "${BIND_HOST:-127.0.0.1}:${SERVER_PORT:-8080}:8080", name)
		require.NotContains(t, content, "${BIND_HOST:-0.0.0.0}:${SERVER_PORT:-8080}:8080", name)
	}

	envExample := readDeploySecurityFile(t, ".env.example")
	require.Contains(t, envExample, "\nBIND_HOST=127.0.0.1\n")
}

func TestProductionComposeRequiresRedisAuthentication(t *testing.T) {
	bundled := readDeploySecurityFile(t, "docker-compose.yml")
	require.GreaterOrEqual(t, strings.Count(bundled, "${REDIS_PASSWORD:?REDIS_PASSWORD is required}"), 3)
	require.Contains(t, bundled, `--requirepass "$${REDIS_PASSWORD}"`)
	require.Contains(t, bundled, "REDISCLI_AUTH=${REDIS_PASSWORD:?REDIS_PASSWORD is required}")
	require.NotContains(t, bundled, `${REDIS_PASSWORD:+--requirepass`)

	standalone := readDeploySecurityFile(t, "docker-compose.standalone.yml")
	require.Contains(t, standalone, "REDIS_PASSWORD=${REDIS_PASSWORD:?REDIS_PASSWORD is required}")

	envExample := readDeploySecurityFile(t, ".env.example")
	require.Contains(t, envExample, "openssl rand -hex 32")
	require.Contains(t, envExample, "docker-compose.local.yml / docker-compose.dev.yml may use no-auth Redis")
}

func TestInstallerFailsClosedWhenChecksumsAreUnavailable(t *testing.T) {
	content := readDeploySecurityFile(t, "install.sh")
	start := strings.Index(content, "download_and_extract()")
	require.GreaterOrEqual(t, start, 0)
	remaining := content[start:]
	end := strings.Index(remaining, "\n# Create system user")
	require.Greater(t, end, 0)
	downloadFunction := remaining[:end]

	require.Contains(t, downloadFunction, `curl -fsSL "$checksum_url"`)
	require.Contains(t, downloadFunction, `if ! [[ "$expected_checksum" =~ ^[[:xdigit:]]{64}$ ]]`)
	require.NotContains(t, downloadFunction, `print_warning "$(msg 'checksum_not_found')"`)
	require.GreaterOrEqual(t, strings.Count(downloadFunction, "checksum_not_found"), 2)
}

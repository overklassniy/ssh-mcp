package status

import (
	"context"
	"regexp"
	"testing"

	"github.com/overklassniy/ssh-mcp/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectSystemStatus_BasicParse(t *testing.T) {
	// The runner simulates output using the marker from the script
	runner := func(ctx context.Context, command, connName string) (string, error) {
		// Extract the marker from the command using regex
		// The script contains: printf '\n__MCP_FIELD_xxx_hostname\n'
		re := regexp.MustCompile(`__MCP_FIELD_[0-9a-f]+_`)
		match := re.FindString(command)
		if match == "" {
			return "", nil
		}
		marker := match

		return "\n" + marker + "hostname\ntesthost\n" +
			"\n" + marker + "osName\nLinux\n" +
			"\n" + marker + "kernelVersion\n5.15.0-91-generic\n" +
			"\n" + marker + "memory\nfree:2.0Gi total:8.0Gi\n", nil
	}

	pol, _ := policy.New(nil, nil)
	result, err := CollectSystemStatus(context.Background(), runner, "test", pol)
	require.NoError(t, err)
	assert.True(t, result.Reachable)
	assert.Equal(t, "testhost", result.Hostname)
	assert.Equal(t, "Linux", result.OSName)
	assert.Equal(t, "5.15.0-91-generic", result.KernelVersion)
	assert.Equal(t, "free:2.0Gi total:8.0Gi", result.Memory)
}

func TestCollectSystemStatus_EmptyOutput(t *testing.T) {
	runner := func(ctx context.Context, command, connName string) (string, error) {
		return "", nil
	}

	pol, _ := policy.New(nil, nil)
	result, err := CollectSystemStatus(context.Background(), runner, "test", pol)
	require.NoError(t, err)
	assert.True(t, result.Reachable)
	assert.Empty(t, result.Hostname)
}

func TestCollectSystemStatus_CommandError(t *testing.T) {
	runner := func(ctx context.Context, command, connName string) (string, error) {
		return "", assert.AnError
	}

	pol, _ := policy.New(nil, nil)
	result, err := CollectSystemStatus(context.Background(), runner, "test", pol)
	require.NoError(t, err)
	assert.True(t, result.Reachable)
	assert.Empty(t, result.Hostname)
}

func TestCollectSystemStatus_WhitelistFilters(t *testing.T) {
	runner := func(ctx context.Context, command, connName string) (string, error) {
		re := regexp.MustCompile(`__MCP_FIELD_[0-9a-f]+_`)
		match := re.FindString(command)
		if match == "" {
			return "", nil
		}
		marker := match
		return "\n" + marker + "hostname\ntesthost\n", nil
	}

	// Whitelist only allows hostname command
	pol, _ := policy.New([]string{`^hostname$`}, nil)
	result, err := CollectSystemStatus(context.Background(), runner, "test", pol)
	require.NoError(t, err)
	assert.Equal(t, "testhost", result.Hostname)
	// Other fields should be empty since their probes were filtered out
	assert.Empty(t, result.OSName)
}

func TestParseStatusScriptOutput(t *testing.T) {
	marker := "__MCP_FIELD_abc_"
	output := "\n__MCP_FIELD_abc_hostname\ntesthost\n" +
		"\n__MCP_FIELD_abc_osName\nLinux\n"

	values := parseStatusScriptOutput(output, marker)
	assert.Equal(t, "testhost", values["hostname"])
	assert.Equal(t, "Linux", values["osName"])
}

func TestBuildStatusScript(t *testing.T) {
	marker := "__MCP_FIELD_abc_"
	commands := map[string]string{
		"hostname": "hostname",
		"osName":   "uname -s",
	}
	script := buildStatusScript(commands, marker)
	assert.Contains(t, script, "hostname")
	assert.Contains(t, script, "uname -s")
	assert.Contains(t, script, marker)
	assert.Contains(t, script, "true")
}

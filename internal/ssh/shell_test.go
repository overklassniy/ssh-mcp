package ssh

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripANSI_Basic(t *testing.T) {
	input := "\x1b[32mgreen text\x1b[0m"
	result := stripANSI(input)
	assert.Equal(t, "green text", result)
}

func TestStripANSI_NoSequences(t *testing.T) {
	input := "plain text"
	result := stripANSI(input)
	assert.Equal(t, "plain text", result)
}

func TestStripANSI_CSIWithNumbers(t *testing.T) {
	input := "\x1b[1;31mbold red\x1b[0m text"
	result := stripANSI(input)
	assert.Equal(t, "bold red text", result)
}

func TestStripANSI_OSCSequence(t *testing.T) {
	input := "\x1b]0;window title\x07hello"
	result := stripANSI(input)
	assert.Equal(t, "hello", result)
}

func TestStripANSI_EmptyString(t *testing.T) {
	assert.Equal(t, "", stripANSI(""))
}

func TestParseExitCode_Valid(t *testing.T) {
	assert.Equal(t, 0, parseExitCode("0__\n"))
	assert.Equal(t, 42, parseExitCode("42__\n"))
	assert.Equal(t, -1, parseExitCode("-1__\n"))
}

func TestParseExitCode_Invalid(t *testing.T) {
	assert.Equal(t, -1, parseExitCode("not a number__\n"))
	assert.Equal(t, -1, parseExitCode(""))
}

func TestBuildShellCommandScript_Basic(t *testing.T) {
	script := buildShellCommandScript("cmd123", "echo hello", "", "")
	assert.Contains(t, script, "__MCP_BEGIN__cmd123__")
	assert.Contains(t, script, "__MCP_END__cmd123__RC__")
	assert.Contains(t, script, "echo hello")
	assert.Contains(t, script, "__mcp_rc=$?")
}

func TestBuildShellCommandScript_WithDirectory(t *testing.T) {
	script := buildShellCommandScript("cmd123", "echo hello", "/tmp", "")
	assert.Contains(t, script, "cd -- '/tmp'")
	assert.Contains(t, script, "echo hello")
}

func TestBuildShellCommandScript_WithTemplate(t *testing.T) {
	script := buildShellCommandScript("cmd123", "echo hello", "", "sudo bash -c <quotedCommand>")
	assert.Contains(t, script, "sudo bash -c")
	// The template wraps the command body (which includes { echo hello; })
	assert.Contains(t, script, "echo hello")
}

func TestApplyCommandTemplate_NoTemplate(t *testing.T) {
	assert.Equal(t, "echo hello", applyCommandTemplate("", "echo hello", ""))
}

func TestApplyCommandTemplate_WithCommand(t *testing.T) {
	result := applyCommandTemplate("wrapper <command>", "echo hello", "")
	assert.Equal(t, "wrapper echo hello", result)
}

func TestApplyCommandTemplate_WithQuotedCommand(t *testing.T) {
	result := applyCommandTemplate("wrapper <quotedCommand>", "echo hello", "")
	assert.Contains(t, result, "wrapper ")
	assert.Contains(t, result, "'echo hello'")
}

func TestShellQuote(t *testing.T) {
	assert.Equal(t, "'hello'", shellQuote("hello"))
	assert.Equal(t, "'it'\\''s'", shellQuote("it's"))
}

package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_EmptyPatterns(t *testing.T) {
	p, err := New(nil, nil)
	require.NoError(t, err)
	assert.False(t, p.HasWhitelist())
	assert.False(t, p.HasBlacklist())
}

func TestNew_InvalidPattern(t *testing.T) {
	_, err := New([]string{"["}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "whitelist")
}

func TestValidate_NoLists_AllowsAll(t *testing.T) {
	p, _ := New(nil, nil)
	allowed, _ := p.Validate("rm -rf /")
	assert.True(t, allowed)
}

func TestValidate_WhitelistOnly(t *testing.T) {
	p, _ := New([]string{`^ls( .*)?`, `^cat .*`}, nil)
	allowed, _ := p.Validate("ls -la")
	assert.True(t, allowed)
	allowed, _ = p.Validate("rm -rf /")
	assert.False(t, allowed)
}

func TestValidate_BlacklistOnly(t *testing.T) {
	p, _ := New(nil, []string{`^rm .*`, `^shutdown`})
	allowed, _ := p.Validate("ls -la")
	assert.True(t, allowed)
	allowed, _ = p.Validate("rm -rf /")
	assert.False(t, allowed)
}

func TestValidate_BothLists_WhitelistFirst(t *testing.T) {
	p, _ := New([]string{`^.*`}, []string{`^rm .*`})
	allowed, _ := p.Validate("ls -la")
	assert.True(t, allowed)
	allowed, reason := p.Validate("rm -rf /")
	assert.False(t, allowed)
	assert.Contains(t, reason, "blacklist")
}

func TestValidate_WhitelistRejectsNotInList(t *testing.T) {
	p, _ := New([]string{`^ls .*`}, nil)
	allowed, reason := p.Validate("cat /etc/passwd")
	assert.False(t, allowed)
	assert.Contains(t, reason, "whitelist")
}

func TestNew_EmptyStringSkipped(t *testing.T) {
	p, err := New([]string{"", `^ls`}, nil)
	require.NoError(t, err)
	assert.True(t, p.HasWhitelist())
	allowed, _ := p.Validate("ls")
	assert.True(t, allowed)
}

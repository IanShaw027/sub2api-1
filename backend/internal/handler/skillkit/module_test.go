//go:build unit

package skillkit

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDefaultRequiresInjectedModule(t *testing.T) {
	defaultModuleMu.Lock()
	previous := defaultModule
	defaultModule = nil
	defaultModuleMu.Unlock()
	t.Cleanup(func() { SetDefault(previous) })

	module, err := Default()
	require.Nil(t, module)
	require.ErrorIs(t, err, service.ErrAISkillServiceUnavailable)

	injected := &Module{}
	SetDefault(injected)
	module, err = Default()
	require.NoError(t, err)
	require.Same(t, injected, module)
}

package extension_test

import (
	"testing"

	"centag/core/pkg/extension"

	"github.com/stretchr/testify/require"
)

type stubPlugin struct {
	name string
	inits int
}

func (s *stubPlugin) Name() string { return s.name }

func (s *stubPlugin) Init(host extension.Host) error {
	s.inits++
	_ = host.Edition()
	return nil
}

func TestRegisterAndInitAll(t *testing.T) {
	extension.ResetForTest()
	defer extension.ResetForTest()

	p := &stubPlugin{name: "demo"}
	extension.Register(p)
	require.Len(t, extension.Plugins(), 1)
	require.NoError(t, extension.InitAll(nil))
	require.Equal(t, 1, p.inits)
}

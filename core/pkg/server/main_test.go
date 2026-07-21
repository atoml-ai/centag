package server

import (
	"os"
	"testing"

	"centag/core/pkg/logger"
)

func TestMain(m *testing.M) {
	// Avoid nil zap.Logger panics in handlers/hooks that log during unit tests.
	_ = logger.Init(logger.Config{
		Level:  "error",
		Format: "console",
		Output: "stdout",
	})
	os.Exit(m.Run())
}

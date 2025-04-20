package storee2e

import (
	"testing"

	"github.com/lstoll/web/session"
	"github.com/lstoll/web/session/kvtest"
)

func TestMemoryKV_E2E(t *testing.T) {
	kv := session.NewMemoryKV()

	kvtest.RunComplianceTest(t, kv, nil)
}

package storee2e

import (
	"testing"

	"lds.li/web/session"
	"lds.li/web/session/kvtest"
)

func TestMemoryKV_E2E(t *testing.T) {
	kv := session.NewMemoryKV()

	kvtest.RunComplianceTest(t, kv, nil)
}

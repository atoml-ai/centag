package conversation

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

func newID(prefix string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return prefix + time.Now().UTC().Format("20060102150405") + hex.EncodeToString(b[:])
}

package app

import (
	"context"
	"testing"

	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/source"
)

func FuzzVerifyLoadedSnapshot(f *testing.F) {
	f.Add([]byte(mustSeedSnapshot(f)))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"schemaVersion":2}`))
	f.Add([]byte{0xff, 0xfe})
	f.Fuzz(func(t *testing.T, data []byte) {
		validated, err := snapshot.LoadBytes(data)
		if err != nil {
			return
		}
		result, err := VerifyValidatedFromSource(context.Background(), validated, source.Memory{Artifact: validated.Artifact})
		if err != nil {
			t.Fatalf("valid snapshot verify: %v", err)
		}
		if result.Status != VerifyMatch {
			t.Fatalf("status = %s", result.Status)
		}
	})
}

func mustSeedSnapshot(f *testing.F) string {
	f.Helper()
	raw := mustSnapshotBytes(f, directoryArtifact(fileNode("a.txt")), snapshot.CaptureV1{Ignore: []string{}})
	return string(raw)
}

package jobs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkCopySmallFiles(b *testing.B) {
	for _, count := range []int{1, 100} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			srcDir, dstDir := b.TempDir(), b.TempDir()
			sources := make([]string, count)
			for i := range sources {
				sources[i] = filepath.Join(srcDir, fmt.Sprintf("file-%d", i))
				if err := os.WriteFile(sources[i], []byte("x"), 0600); err != nil {
					b.Fatal(err)
				}
			}
			manager := &Manager{}
			job := &Job{Type: TypeCopy, ctx: b.Context(), Sources: sources, DestDir: dstDir, Resolver: func(context.Context, ConflictRequest) ConflictResolution {
				return ConflictResolution{Action: ConflictOverwrite}
			}}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := manager.runJob(job); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

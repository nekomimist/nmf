package jobs

import (
	"fmt"
	"testing"
)

var benchmarkRemainingJobs int

func BenchmarkJobIndicator(b *testing.B) {
	for _, sources := range []int{100, 10000} {
		for _, windows := range []int{1, 4} {
			b.Run(fmt.Sprintf("sources%d_windows%d", sources, windows), func(b *testing.B) {
				paths := make([]string, sources)
				for i := range paths {
					paths[i] = fmt.Sprintf("/source/file-%05d", i)
				}
				m := &Manager{current: &Job{Status: StatusRunning, Sources: paths}, queue: []*Job{{Status: StatusPending, Sources: paths}}}
				for range 100 {
					m.history = append(m.history, &Job{Status: StatusCompleted, Sources: paths})
				}
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					for range windows {
						summary := m.Summary()
						benchmarkRemainingJobs = summary.Pending + summary.Running
					}
				}
			})
		}
	}
}

package pool

import (
	"testing"
)

func BenchmarkUnbufferedJ10W5(b *testing.B) {
	benchmarkDispatcherPipeline(b, 10, 5, false)
}

func BenchmarkUnbufferedJ100W10(b *testing.B) {
	benchmarkDispatcherPipeline(b, 100, 10, false)
}

func BenchmarkUnbufferedJ1000W10(b *testing.B) {
	benchmarkDispatcherPipeline(b, 1000, 10, false)
}

func benchmarkDispatcherPipeline(b *testing.B, j, w int, buffered bool) {
	for n := 0; n < b.N; n++ {
		_, _ = testDispatcherPipeline(j, w, buffered, &testWorker{1}, testWorkerHandler)
	}
}

func BenchmarkBufferedJ10W5(b *testing.B) {
	benchmarkDispatcherPipeline(b, 10, 5, true)
}

func BenchmarkBufferedJ100W10(b *testing.B) {
	benchmarkDispatcherPipeline(b, 100, 10, true)
}

func BenchmarkBufferedJ1000W10(b *testing.B) {
	benchmarkDispatcherPipeline(b, 1000, 10, true)
}

func benchmarkCustomPipeline(b *testing.B, j, w int) {
	for n := 0; n < b.N; n++ {
		_ = testCustomPipeline(j, w, &testWorker{1}, testWorkerHandler)
	}
}

func BenchmarkCustomJ10W5(b *testing.B) {
	benchmarkCustomPipeline(b, 10, 5)
}

func BenchmarkCustomJ100W10(b *testing.B) {
	benchmarkCustomPipeline(b, 100, 10)
}

func BenchmarkCustomJ1000W10(b *testing.B) {
	benchmarkCustomPipeline(b, 1000, 10)
}

func benchmarkCustomPipelineHW(b *testing.B, j, w int) {
	hw := genHeavyWorker()
	for n := 0; n < b.N; n++ {
		_ = testCustomPipeline(j, w, hw, heavyWorkerHandler)
	}
}

func BenchmarkCustomJ10W5HW(b *testing.B) {
	benchmarkCustomPipelineHW(b, 10, 5)
}

func BenchmarkCustomJ100W10HW(b *testing.B) {
	benchmarkCustomPipelineHW(b, 100, 10)
}

func BenchmarkCustomJ1000W10HW(b *testing.B) {
	benchmarkCustomPipelineHW(b, 1000, 10)
}

func benchmarkDispatcherPipelineHW(b *testing.B, j, w int, buffered bool) {
	hw := genHeavyWorker()
	for n := 0; n < b.N; n++ {
		_, _ = testDispatcherPipeline(j, w, buffered, hw, heavyWorkerHandler)
	}
}

func BenchmarkUnbufferedJ10W5HW(b *testing.B) {
	benchmarkDispatcherPipelineHW(b, 10, 5, false)
}

func BenchmarkUnbufferedJ100W10HW(b *testing.B) {
	benchmarkDispatcherPipelineHW(b, 100, 10, false)
}

func BenchmarkUnbufferedJ1000W10HW(b *testing.B) {
	benchmarkDispatcherPipelineHW(b, 1000, 10, false)
}

func BenchmarkBufferedJ10W5HW(b *testing.B) {
	benchmarkDispatcherPipelineHW(b, 10, 5, true)
}

func BenchmarkBufferedJ100W10HW(b *testing.B) {
	benchmarkDispatcherPipelineHW(b, 100, 10, true)
}

func BenchmarkBufferedJ1000W10HW(b *testing.B) {
	benchmarkDispatcherPipelineHW(b, 1000, 10, true)
}

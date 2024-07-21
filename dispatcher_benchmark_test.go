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
		_, _ = testDispatcherPipeline(j, buffered, &testWorker{1}, testWorkerHandler, WithWorkers(w))
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

func benchmarkGenericPipeline(b *testing.B, j, w int) {
	for n := 0; n < b.N; n++ {
		_ = testGenericPipeline(j, w, &testWorker{1}, testWorkerHandler)
	}
}

func BenchmarkGenericJ10W5(b *testing.B) {
	benchmarkGenericPipeline(b, 10, 5)
}

func BenchmarkGenericJ100W10(b *testing.B) {
	benchmarkGenericPipeline(b, 100, 10)
}

func BenchmarkGenericJ1000W10(b *testing.B) {
	benchmarkGenericPipeline(b, 1000, 10)
}

func benchmarkGenericPipelineHW(b *testing.B, j, w int) {
	hw := genHeavyWorker()
	for n := 0; n < b.N; n++ {
		_ = testGenericPipeline(j, w, hw, heavyWorkerHandler)
	}
}

func BenchmarkGenericJ10W5HW(b *testing.B) {
	benchmarkGenericPipelineHW(b, 10, 5)
}

func BenchmarkGenericJ100W10HW(b *testing.B) {
	benchmarkGenericPipelineHW(b, 100, 10)
}

func BenchmarkGenericJ1000W10HW(b *testing.B) {
	benchmarkGenericPipelineHW(b, 1000, 10)
}

func benchmarkDispatcherPipelineHW(b *testing.B, j, w int, buffered bool) {
	hw := genHeavyWorker()
	for n := 0; n < b.N; n++ {
		_, _ = testDispatcherPipeline(j, buffered, hw, heavyWorkerHandler, WithWorkers(w))
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

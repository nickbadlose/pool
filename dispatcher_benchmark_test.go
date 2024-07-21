package pool

import (
	"testing"
)

func benchmarkGenericPipeline6Steps(b *testing.B, j, w int) {
	hw := genHeavyWorker()
	for n := 0; n < b.N; n++ {
		_ = testGenericPipeline6Steps(j, w, hw, heavyWorkerHandler)
	}
}

func BenchmarkGeneric6StepsJ10W5(b *testing.B) {
	benchmarkGenericPipeline6Steps(b, 10, 5)
}

func BenchmarkGeneric6StepsJ100W10(b *testing.B) {
	benchmarkGenericPipeline6Steps(b, 100, 10)
}

func BenchmarkGeneric6StepsJ1000W10(b *testing.B) {
	benchmarkGenericPipeline6Steps(b, 1000, 10)
}

func benchmarkGenericPipeline3Steps(b *testing.B, j, w int) {
	hw := genHeavyWorker()
	for n := 0; n < b.N; n++ {
		_ = testGenericPipeline3Steps(j, w, hw, heavyWorkerHandler)
	}
}

func BenchmarkGeneric3StepsJ10W5(b *testing.B) {
	benchmarkGenericPipeline3Steps(b, 10, 5)
}

func BenchmarkGeneric3StepsJ100W10(b *testing.B) {
	benchmarkGenericPipeline3Steps(b, 100, 10)
}

func BenchmarkGeneric3StepsJ1000W10(b *testing.B) {
	benchmarkGenericPipeline3Steps(b, 1000, 10)
}

func benchmarkDispatcherPipeline(b *testing.B, j, w int, buffered bool) {
	hw := genHeavyWorker()
	for n := 0; n < b.N; n++ {
		_, _ = testDispatcherPipeline(j, buffered, hw, heavyWorkerHandler, WithWorkers(w))
	}
}

func BenchmarkUnbufferedJ10W5(b *testing.B) {
	benchmarkDispatcherPipeline(b, 10, 5, false)
}

func BenchmarkUnbufferedJ100W10(b *testing.B) {
	benchmarkDispatcherPipeline(b, 100, 10, false)
}

func BenchmarkUnbufferedJ1000W10(b *testing.B) {
	benchmarkDispatcherPipeline(b, 1000, 10, false)
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

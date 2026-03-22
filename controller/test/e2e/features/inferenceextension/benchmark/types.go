package benchmark

import "time"

type BenchmarkConfig struct {
	KubeTarget  string
	Namespace   string
	LocalPort   int 
	ServicePort int
	Model       string
	Concurrency int
	Duration    time.Duration
	OutputDir  	string
}

type BenchmarkResult struct {
	Stack      string
	P50Latency float64
	P90Latency float64
	P99Latency float64
	Throughput float64
	TTFT       float64
	TPOT       float64
	ITL        float64
}

type Overhead struct {
	P50DeltaMs  float64
	P90DeltaMs  float64
	P99DeltaMs  float64
	TTFTDeltaMs float64
}

func ComputeOverhead(baseline, agw BenchmarkResult) Overhead {
	return Overhead{
		P50DeltaMs:  agw.P50Latency - baseline.P50Latency,
		P90DeltaMs:  agw.P90Latency - baseline.P90Latency,
		P99DeltaMs:  agw.P99Latency - baseline.P99Latency,
		TTFTDeltaMs: agw.TTFT - baseline.TTFT,
	}
}

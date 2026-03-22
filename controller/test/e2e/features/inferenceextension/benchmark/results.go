package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func ParseResults(dir, stack string) (BenchmarkResult, error) {
	path := filepath.Join(dir, "summary_lifecycle_metrics.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return BenchmarkResult{}, fmt.Errorf("reading results file: %w", err)
	}

	var raw struct {
		Successes struct {
            Latency struct {
                RequestLatency struct {
                    Median float64 `json:"median"`
                    P90    float64 `json:"p90"`
                    P99    float64 `json:"p99"`
                } `json:"request_latency"`
                TTFT struct {
                    Median float64 `json:"median"`
                    P90    float64 `json:"p90"`
                } `json:"time_to_first_token"`
                TPOT struct {
                    Mean float64 `json:"mean"`
                } `json:"time_per_output_token"`
                ITL struct {
                    Mean   float64 `json:"mean"`
                    Median float64 `json:"median"`
                } `json:"inter_token_latency"`
            } `json:"latency"`
            Throughput struct {
                RequestsPerSec     float64 `json:"requests_per_sec"`
                OutputTokensPerSec float64 `json:"output_tokens_per_sec"`
            } `json:"throughput"`
        } `json:"successes"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return BenchmarkResult{}, fmt.Errorf("parsing result JSON: %w", err)
	}

	return BenchmarkResult{
		Stack:      stack,
		P50Latency: raw.Successes.Latency.RequestLatency.Median,
		P90Latency: raw.Successes.Latency.RequestLatency.P90,
		P99Latency: raw.Successes.Latency.RequestLatency.P99,
		Throughput: raw.Successes.Throughput.RequestsPerSec,
		TTFT:       raw.Successes.Latency.TTFT.Median,
		TPOT:       raw.Successes.Latency.TPOT.Mean,
		ITL:        raw.Successes.Latency.ITL.Mean,
	}, nil
}

func PrintComparison(baseline, agw BenchmarkResult) {
	oh := ComputeOverhead(baseline, agw)
	fmt.Printf("\n=== Benchmark Results ===\n")
	fmt.Printf("%-20s %-15s %-15s %-10s\n", "Metric", "Baseline", "agentgateway", "Overhead")
	fmt.Printf("%-20s %-15.2f %-15.2f %+.2f ms\n", "P50 Latency (ms)", baseline.P50Latency, agw.P50Latency, oh.P50DeltaMs)
	fmt.Printf("%-20s %-15.2f %-15.2f %+.2f ms\n", "P95 Latency (ms)", baseline.P90Latency, agw.P90Latency, oh.P90DeltaMs)
	fmt.Printf("%-20s %-15.2f %-15.2f %+.2f ms\n", "P99 Latency (ms)", baseline.P99Latency, agw.P99Latency, oh.P99DeltaMs)
	fmt.Printf("%-20s %-15.2f %-15.2f %+.2f ms\n", "TTFT (ms)", baseline.TTFT, agw.TTFT, oh.TTFTDeltaMs)
	fmt.Printf("%-20s %-15.2f %-15.2f %s\n", "Throughput (rps)", baseline.Throughput, agw.Throughput, "(higher=better)")
	fmt.Printf("\nNOTE: ITL (Inter-Token Latency) not yet instrumented in agentgateway.\n")
	fmt.Printf("See: crates/agentgateway/src/telemetry/log.rs add_llm_metrics()\n")
}

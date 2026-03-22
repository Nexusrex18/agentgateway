//go:build e2e

package benchmark

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
	"path/filepath"
	"runtime"

	e2e "github.com/agentgateway/agentgateway/controller/test/e2e"
	"github.com/stretchr/testify/suite"
)

var _ suite.TestingSuite = new(BenchmarkSuite)

var testdataDir = func() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "testdata")
}()

type BenchmarkSuite struct {
	suite.Suite
	ctx context.Context
	ti *e2e.TestInstallation
}

func NewBenchmarkSuite(ctx context.Context, ti *e2e.TestInstallation) suite.TestingSuite {
	return &BenchmarkSuite{
		ctx: ctx,
		ti:  ti,
	}
}

func (s *BenchmarkSuite) SetupSuite() {
	manifest := []string{
		"llm-sim.yaml",
        "baseline-svc.yaml",
        "httproute.yaml",
        "referencegrant.yaml",
	}

	for _, m := range manifest {
		cmd := exec.CommandContext(s.ctx, "kubectl", "apply", "-f",
			filepath.Join(testdataDir, m))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		s.Require().NoError(cmd.Run())
	}

	exec.CommandContext(s.ctx, "kubectl", "rollout", "status",
        "deployment/llm-sim", "-n", "benchmark", "--timeout=60s").Run()
}

func (s *BenchmarkSuite) TestBaselineVsAgentgateway() {
	baselineCfg := BenchmarkConfig{
		KubeTarget:  	"svc/llm-sim-svc",
    	Namespace:   	"benchmark",
		LocalPort: 		18000,
		ServicePort: 	8000,
		Model:      	"TinyLlama/TinyLlama-1.1B-Chat-v1.0",
		Concurrency: 	10,
		Duration:    	60 * time.Second,
		OutputDir:  	s.ti.GeneratedFiles.TempDir + "/baseline-results",
	}

	agwCfg := BenchmarkConfig{
		KubeTarget:  	"svc/gateway",
    	Namespace:   	"agentgateway-base",
		LocalPort: 		18001,
		ServicePort: 	80,
		Model:      	"TinyLlama/TinyLlama-1.1B-Chat-v1.0",
		Concurrency: 	10,
		Duration:    	60 * time.Second,
		OutputDir:  	s.ti.GeneratedFiles.TempDir + "/agw-results",
	}

	s.T().Run("baseline", func(t *testing.T) {
		runInferencePerf(t, s.ctx, baselineCfg)
	})

	s.T().Run("agentgateway", func(t *testing.T) {
		runInferencePerf(t, s.ctx, agwCfg)
	})

	baseline, err := ParseResults(baselineCfg.OutputDir, "baseline")
	s.Require().NoError(err)

	agw, err := ParseResults(agwCfg.OutputDir, "agentgateway")
	s.Require().NoError(err)

	PrintComparison(baseline, agw)

	oh := ComputeOverhead(baseline, agw)
	const maxP99OverheadMs = 5.0
	s.Require().LessOrEqualf(oh.P99DeltaMs, maxP99OverheadMs,
		"p99 overhead %.2fms exceeds threshold %.2fms — possible regression",
		oh.P99DeltaMs, maxP99OverheadMs)

}

func runInferencePerf(t *testing.T, ctx context.Context, cfg BenchmarkConfig) {
	t.Helper()

	config := fmt.Sprintf(`data:
  type: mock
load:
  type: constant
  stages:
  - rate: %d
    duration: %.0f
api:
  type: chat
  streaming: true
server:
  type: vllm
  model_name: %s
  base_url: http://localhost:%d
  ignore_eos: true
storage:
  local_storage:
    path: %s
`, cfg.Concurrency, cfg.Duration.Seconds(), cfg.Model, cfg.LocalPort, cfg.OutputDir)

	configFile, err := os.CreateTemp("", "inference-perf-*.yml")
	if err != nil {
		t.Fatalf("failed to create inference-perf config: %v", err)
	}
	defer os.Remove(configFile.Name())

	if _, err := configFile.WriteString(config); err != nil {
		t.Fatalf("failed to write inference-perf config: %v", err)
	}
	configFile.Close()

	pf := exec.CommandContext(ctx, "kubectl", "port-forward",
    	"-n", cfg.Namespace,
    	cfg.KubeTarget,
    	fmt.Sprintf("%d:%d", cfg.LocalPort, cfg.ServicePort),
	)
	pf.Stdout = os.Stdout
	pf.Stderr = os.Stderr
	if err := pf.Start(); err != nil {
    	t.Fatalf("port-forward failed: %v", err)
	}
	defer pf.Process.Kill()
	time.Sleep(2 * time.Second)

	cmd := exec.CommandContext(ctx, "inference-perf", "-c", configFile.Name())
	cmd.Env = append(os.Environ(), "PYENV_VERSION=3.14.3")	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("inference-perf failed: %v", err)
	}
}

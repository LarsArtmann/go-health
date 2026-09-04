// Command doanalyzerv2-runner runs the private go-design-smells doanalyzerv2
// AST analyzer against a target directory (default: the go-health repo root)
// and reports samber/do v2 anti-pattern findings (DO-1..DO-6) as
// file:line diagnostics. Exit code 1 means findings, 0 means clean.
//
// It exists because the analyzer lives in a private repo that the nix
// sandbox cannot fetch, so it cannot become a flake app; instead it runs as
// a library via the local replace directive in go.mod. See CONTRIBUTING.md.
//
// Usage (from the repo root or tools/doanalyzerv2):
//
//	go run . /path/to/target
package main

import (
	"fmt"
	"os"
	"path/filepath"

	stats "github.com/larsartmann/go-design-smells/pkg/stats"
)

func main() {
	target := "."
	if len(os.Args) > 1 {
		target = os.Args[1]
	}

	abs, err := filepath.Abs(target)
	if err != nil {
		fatalf("resolve target %q: %v", target, err)
	}

	detections, err := stats.CollectDoAnalyzerV2Violations(abs, stats.Options{
		ExcludeGenerated: true,
	})
	if err != nil {
		fatalf("analyze %s: %v", abs, err)
	}

	for _, d := range detections {
		fmt.Printf("%s:%d: [%s] %s\n  %s\n", d.FilePath, d.Line, d.PatternName(), d.Message, d.Suggestion)
	}

	fmt.Printf("doanalyzerv2: %d finding(s)\n", len(detections))

	if len(detections) > 0 {
		os.Exit(1)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "doanalyzerv2-runner: "+format+"\n", args...)
	os.Exit(2)
}

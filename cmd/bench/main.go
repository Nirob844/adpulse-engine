package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ANSI color codes for rich terminal styling
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorGray   = "\033[90m"
	colorPurple = "\033[35m"
)

type Config struct {
	BaseURL     string
	TotalReqs   int
	Concurrency int
	Scenario    string
	Timeout     time.Duration
}

type Result struct {
	TotalDuration time.Duration
	Latencies     []time.Duration
	Status200     int64
	Status204     int64
	Status302     int64
	StatusOther   int64
	Errors        int64
}

func main() {
	var cfg Config
	flag.StringVar(&cfg.BaseURL, "url", "http://localhost:8000", "Target Ad Server base URL")
	flag.IntVar(&cfg.TotalReqs, "requests", 5000, "Total number of requests to execute")
	flag.IntVar(&cfg.TotalReqs, "n", 5000, "Alias for -requests")
	flag.IntVar(&cfg.Concurrency, "concurrency", 50, "Number of concurrent worker goroutines")
	flag.IntVar(&cfg.Concurrency, "c", 50, "Alias for -concurrency")
	flag.StringVar(&cfg.Scenario, "scenario", "mixed", "Workload scenario: 'mixed', 'ad', 'impression', 'click'")
	flag.StringVar(&cfg.Scenario, "s", "mixed", "Alias for -scenario")
	flag.DurationVar(&cfg.Timeout, "timeout", 5*time.Second, "Per-request HTTP client timeout")
	flag.Parse()

	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	printHeader(cfg)

	// 1. Initial healthcheck verification
	if err := checkHealth(cfg.BaseURL, cfg.Timeout); err != nil {
		fmt.Printf("\n%s[ERROR]%s Cannot reach Ad Server at %s: %v\n", colorRed, colorReset, cfg.BaseURL, err)
		fmt.Println("Please make sure the server is running (e.g. 'go run ./cmd/server' or 'make run').")
		os.Exit(1)
	}
	fmt.Printf(" %s[OK]%s Target server is healthy (PostgreSQL & Redis connected)\n", colorGreen, colorReset)
	fmt.Println(" Starting load test...")

	// 2. Execute benchmark
	res := runBenchmark(cfg)

	// 3. Print comprehensive report
	printReport(cfg, res)
}

func checkHealth(baseURL string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}
	return nil
}

func runBenchmark(cfg Config) Result {
	// High-performance pooled HTTP transport
	transport := &http.Transport{
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 1000,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
		DisableKeepAlives:   false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Do not follow redirects so we measure pure 302 response latency
			return http.ErrUseLastResponse
		},
	}

	var status200, status204, status302, statusOther, errCount atomic.Int64
	var completed atomic.Int64

	taskChan := make(chan int, cfg.TotalReqs)
	for i := 0; i < cfg.TotalReqs; i++ {
		taskChan <- i
	}
	close(taskChan)

	var wg sync.WaitGroup
	workerLatencies := make([][]time.Duration, cfg.Concurrency)

	startTime := time.Now()

	// Progress bar ticker
	doneChan := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-doneChan:
				return
			case <-ticker.C:
				done := completed.Load()
				elapsed := time.Since(startTime).Seconds()
				rps := 0.0
				if elapsed > 0 {
					rps = float64(done) / elapsed
				}
				percent := float64(done) / float64(cfg.TotalReqs) * 100
				bars := int(percent / 5)
				barStr := strings.Repeat("█", bars) + strings.Repeat("░", 20-bars)
				fmt.Printf("\r %s[%s]%s %5.1f%% (%d/%d) %s|%s RPS: %.0f req/s",
					colorCyan, barStr, colorReset, percent, done, cfg.TotalReqs, colorGray, colorReset, rps)
			}
		}
	}()

	// Spawn concurrent workers
	for w := 0; w < cfg.Concurrency; w++ {
		workerID := w
		wg.Add(1)
		go func(wID int) {
			defer wg.Done()
			lats := make([]time.Duration, 0, cfg.TotalReqs/cfg.Concurrency+10)

			sizes := []string{"728x90", "300x250"}
			creativeIDs := []int{1, 2, 3, 4, 5, 6}

			for i := range taskChan {
				url := buildRequestURL(cfg.BaseURL, cfg.Scenario, i, sizes, creativeIDs)

				reqStart := time.Now()
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					errCount.Add(1)
					completed.Add(1)
					continue
				}

				// Rotate simulated client IP
				simIP := fmt.Sprintf("192.168.1.%d", (i%250)+1)
				req.Header.Set("X-Forwarded-For", simIP)
				req.Header.Set("User-Agent", "MiniAdServer-Benchmark/1.0 (HighConcurrency; Linux)")

				resp, err := client.Do(req)
				dur := time.Since(reqStart)
				lats = append(lats, dur)

				if err != nil {
					errCount.Add(1)
				} else {
					switch resp.StatusCode {
					case http.StatusOK:
						status200.Add(1)
					case http.StatusNoContent:
						status204.Add(1)
					case http.StatusFound, http.StatusMovedPermanently, http.StatusSeeOther:
						status302.Add(1)
					default:
						statusOther.Add(1)
					}
					_ = resp.Body.Close()
				}
				completed.Add(1)
			}
			workerLatencies[wID] = lats
		}(workerID)
	}

	wg.Wait()
	close(doneChan)
	totalDuration := time.Since(startTime)

	// Clean progress line
	fmt.Printf("\r %s[████████████████████]%s 100.0%% (%d/%d) %s|%s Completed in %s\n\n",
		colorGreen, colorReset, cfg.TotalReqs, cfg.TotalReqs, colorGray, colorReset, formatDuration(totalDuration))

	// Aggregate and sort latencies
	totalLats := make([]time.Duration, 0, cfg.TotalReqs)
	for _, lats := range workerLatencies {
		totalLats = append(totalLats, lats...)
	}
	sort.Slice(totalLats, func(i, j int) bool {
		return totalLats[i] < totalLats[j]
	})

	return Result{
		TotalDuration: totalDuration,
		Latencies:     totalLats,
		Status200:     status200.Load(),
		Status204:     status204.Load(),
		Status302:     status302.Load(),
		StatusOther:   statusOther.Load(),
		Errors:        errCount.Load(),
	}
}

func buildRequestURL(baseURL, scenario string, idx int, sizes []string, creativeIDs []int) string {
	switch scenario {
	case "ad":
		size := sizes[idx%len(sizes)]
		return fmt.Sprintf("%s/ad?size=%s", baseURL, size)
	case "impression":
		cID := creativeIDs[idx%len(creativeIDs)]
		return fmt.Sprintf("%s/track/impression?c=%d", baseURL, cID)
	case "click":
		cID := creativeIDs[idx%len(creativeIDs)]
		return fmt.Sprintf("%s/track/click?c=%d", baseURL, cID)
	default: // "mixed"
		// 70% Ad Serving, 25% Impressions, 5% Clicks (standard digital ad conversion ratio)
		roll := rand.Intn(100)
		if roll < 70 {
			size := sizes[idx%len(sizes)]
			return fmt.Sprintf("%s/ad?size=%s", baseURL, size)
		} else if roll < 95 {
			cID := creativeIDs[idx%len(creativeIDs)]
			return fmt.Sprintf("%s/track/impression?c=%d", baseURL, cID)
		} else {
			cID := creativeIDs[idx%len(creativeIDs)]
			return fmt.Sprintf("%s/track/click?c=%d", baseURL, cID)
		}
	}
}

func printHeader(cfg Config) {
	fmt.Println()
	fmt.Printf("%s╔══════════════════════════════════════════════════════════════════════════╗%s\n", colorCyan, colorReset)
	fmt.Printf("%s║          ADPULSE ENGINE - HIGH-CONCURRENCY BENCHMARK TOOL                ║%s\n", colorCyan, colorReset)
	fmt.Printf("%s╚══════════════════════════════════════════════════════════════════════════╝%s\n", colorCyan, colorReset)
	fmt.Printf(" Target URL     : %s%s%s\n", colorBold, cfg.BaseURL, colorReset)
	fmt.Printf(" Workload Mode  : %s%s%s\n", colorYellow, strings.ToUpper(cfg.Scenario), colorReset)
	fmt.Printf(" Requests Count : %s%d%s\n", colorBold, cfg.TotalReqs, colorReset)
	fmt.Printf(" Concurrency    : %s%d workers%s\n", colorBold, cfg.Concurrency, colorReset)
	fmt.Println("──────────────────────────────────────────────────────────────────────────")
}

func printReport(cfg Config, res Result) {
	rps := 0.0
	if res.TotalDuration.Seconds() > 0 {
		rps = float64(cfg.TotalReqs) / res.TotalDuration.Seconds()
	}

	p50, p90, p95, p99, minLat, maxLat, avgLat := calculateStats(res.Latencies)

	fmt.Printf("%s========================== BENCHMARK RESULTS ============================%s\n", colorCyan, colorReset)
	fmt.Printf(" Total Requests      : %s%d%s\n", colorBold, cfg.TotalReqs, colorReset)
	fmt.Printf(" Concurrency Level   : %d workers\n", cfg.Concurrency)
	fmt.Printf(" Total Time Taken    : %s%s%s\n", colorBold, formatDuration(res.TotalDuration), colorReset)
	fmt.Printf(" Throughput (RPS)    : %s%s%.2f requests/sec%s 🚀\n", colorBold, colorGreen, rps, colorReset)

	fmt.Println()
	fmt.Printf("%s---------------------------- LATENCY PROFILE ----------------------------%s\n", colorCyan, colorReset)
	fmt.Printf(" Min Latency         : %s\n", formatDuration(minLat))
	fmt.Printf(" Average (Mean)      : %s\n", formatDuration(avgLat))
	fmt.Printf(" 50%% (Median P50)    : %s%s%s\n", colorBold, formatDuration(p50), colorReset)
	fmt.Printf(" 90%% (P90)           : %s\n", formatDuration(p90))
	fmt.Printf(" 95%% (P95)           : %s%s%s\n", colorBold, formatDuration(p95), colorReset)
	fmt.Printf(" 99%% (P99)           : %s%s%s\n", colorBold, formatDuration(p99), colorReset)
	fmt.Printf(" Max Latency         : %s\n", formatDuration(maxLat))

	fmt.Println()
	fmt.Printf("%s------------------------ HTTP STATUS BREAKDOWN --------------------------%s\n", colorCyan, colorReset)
	fmt.Printf(" 200 OK              : %s%d%s (%.1f%%)\n", colorGreen, res.Status200, colorReset, percentOf(res.Status200, cfg.TotalReqs))
	fmt.Printf(" 204 No Content      : %s%d%s (%.1f%%) - Frequency cap / limit reached\n", colorGray, res.Status204, colorReset, percentOf(res.Status204, cfg.TotalReqs))
	fmt.Printf(" 302 Found           : %s%d%s (%.1f%%) - Click redirects\n", colorPurple, res.Status302, colorReset, percentOf(res.Status302, cfg.TotalReqs))
	if res.StatusOther > 0 {
		fmt.Printf(" Other Statuses      : %s%d%s\n", colorYellow, res.StatusOther, colorReset)
	}
	if res.Errors > 0 {
		fmt.Printf(" Failed / Errors     : %s%d (%.2f%%)%s\n", colorRed, res.Errors, percentOf(res.Errors, cfg.TotalReqs), colorReset)
	} else {
		fmt.Printf(" Failed / Errors     : %s0 (0.00%% Error Rate - 100%% SUCCESS)%s\n", colorGreen, colorReset)
	}

	fmt.Printf("%s==========================================================================%s\n\n", colorCyan, colorReset)
}

func calculateStats(lats []time.Duration) (p50, p90, p95, p99, min, max, avg time.Duration) {
	n := len(lats)
	if n == 0 {
		return
	}

	min = lats[0]
	max = lats[n-1]

	var sum int64
	for _, l := range lats {
		sum += int64(l)
	}
	avg = time.Duration(sum / int64(n))

	p50 = lats[int(float64(n)*0.50)]
	p90 = lats[int(float64(n)*0.90)]
	p95 = lats[int(float64(n)*0.95)]
	p99 = lats[int(float64(n)*0.99)]
	return
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2f µs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2f ms", float64(d.Nanoseconds())/1e6)
	}
	return fmt.Sprintf("%.2f s", d.Seconds())
}

func percentOf(part int64, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

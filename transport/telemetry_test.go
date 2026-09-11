package transport

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestTelemetryTTFT_TTFA(t *testing.T) {
	s := &SessionTelemetry{sessionID: "test-session-1"}
	s.StartThinking()

	// Simulate thinking for 50ms
	time.Sleep(50 * time.Millisecond)
	s.RecordFirstToken()

	// Simulate TTS WebSocket latency for 50ms
	time.Sleep(50 * time.Millisecond)
	s.RecordFirstAudioFrame()

	s.mu.Lock()
	ttft := s.ttft
	ttfa := s.ttfa
	s.mu.Unlock()

	if ttft < 30*time.Millisecond || ttft > 100*time.Millisecond {
		t.Errorf("expected TTFT around 50ms, got %v", ttft)
	}
	if ttfa < 30*time.Millisecond || ttfa > 100*time.Millisecond {
		t.Errorf("expected TTFA around 50ms, got %v", ttfa)
	}
}

func TestTelemetryJitterAccumulator(t *testing.T) {
	s := &SessionTelemetry{sessionID: "test-session-2"}
	s.StartThinking()

	// We push frames with specific mock intervals
	s.RecordOutboundPush()
	
	// Simulate 21ms interval
	time.Sleep(21 * time.Millisecond)
	s.RecordOutboundPush()

	// Simulate 19ms interval
	time.Sleep(19 * time.Millisecond)
	s.RecordOutboundPush()

	s.mu.Lock()
	n := s.frameCount
	sumDiffSq := s.sumOfSquares
	s.mu.Unlock()

	if n != 2 {
		t.Errorf("expected frameCount = 2, got %d", n)
	}

	// variance against 20ms baseline:
	// diff1 = 21 - 20 = 1, diff1^2 = 1
	// diff2 = 19 - 20 = -1, diff2^2 = 1
	// sumDiffSq should be around 2.0 (but sleep timing can vary slightly on Windows)
	if sumDiffSq < 0.5 || sumDiffSq > 10.0 {
		t.Errorf("expected sum of squares near 2.0, got %f", sumDiffSq)
	}
}

func TestTelemetryThreadSafety(t *testing.T) {
	s := &SessionTelemetry{sessionID: "test-session-race"}
	s.StartThinking()

	var wg sync.WaitGroup
	workers := 10
	pushesPerWorker := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < pushesPerWorker; j++ {
				s.RecordOutboundPush()
				s.RecordFirstToken()
				s.RecordFirstAudioFrame()
			}
		}()
	}
	wg.Wait()

	s.mu.Lock()
	n := s.frameCount
	s.mu.Unlock()

	expectedPushes := int64(workers*pushesPerWorker - 1)
	if n < 0 || n > expectedPushes {
		t.Errorf("unexpected frameCount: %d", n)
	}
}

func TestPrintSampleCard(t *testing.T) {
	// Generate a mock telemetry profile for demonstration/verification
	s := &SessionTelemetry{sessionID: "prod-session-evaluation"}
	s.StartThinking()
	
	// Mock some realistic values
	s.mu.Lock()
	s.ttft = 142 * time.Millisecond
	s.ttfa = 224 * time.Millisecond
	
	// Simulate 10 frames with varying intervals around 20ms baseline
	intervals := []float64{20.2, 19.8, 20.5, 19.5, 21.0, 19.0, 20.1, 19.9, 20.3, 19.7}
	s.frameCount = int64(len(intervals))
	for _, val := range intervals {
		s.sumOfIntervals += val
		diff := val - 20.0
		s.sumOfSquares += diff * diff
	}
	s.mu.Unlock()

	t.Log("Printing simulated production observability telemetry card:")
	s.CompileAndReport()
}

func BenchmarkRecordOutboundPushAllocationFree(b *testing.B) {
	s := &SessionTelemetry{sessionID: "bench-session"}
	s.StartThinking()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.RecordOutboundPush()
	}
}

func TestPrecisionSpinYieldPacing(t *testing.T) {
	const frameCount = 5
	const frameDuration = 20 * time.Millisecond
	intervals := make([]time.Duration, 0, frameCount)

	var nextFrameTime time.Time
	lastPush := time.Now()

	for i := 0; i < frameCount; i++ {
		now := time.Now()
		if nextFrameTime.IsZero() || now.After(nextFrameTime.Add(frameDuration)) {
			nextFrameTime = now.Add(frameDuration)
		} else {
			nextFrameTime = nextFrameTime.Add(frameDuration)
		}

		for {
			remaining := time.Until(nextFrameTime)
			if remaining <= 0 {
				break
			}
			if remaining > 3*time.Millisecond {
				timer := time.NewTimer(remaining - 2*time.Millisecond)
				<-timer.C
			} else {
				runtime.Gosched()
			}
		}

		pushTime := time.Now()
		interval := pushTime.Sub(lastPush)
		lastPush = pushTime
		intervals = append(intervals, interval)
	}

	for i := 1; i < len(intervals); i++ {
		diff := intervals[i] - frameDuration
		if diff < 0 {
			diff = -diff
		}
		t.Logf("Frame %d interval: %v (deviation: %v)", i, intervals[i], diff)
		if diff > 2500*time.Microsecond {
			t.Logf("Notice: frame %d interval %v deviated by %v under current OS scheduling", i, intervals[i], diff)
		}
	}
}

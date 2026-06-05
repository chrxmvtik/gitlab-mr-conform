package server

import (
	"fmt"
	"net/http"
	_ "net/http/pprof" // registers /debug/pprof/* on DefaultServeMux
	"runtime"
)

// StartDebugServer starts a pprof HTTP server on the given port.
// It must be called in a separate goroutine.
//
// Available endpoints:
//
//	/debug/pprof/profile?seconds=N  — CPU flame graph (30s is a good default)
//	/debug/pprof/heap               — memory allocation profile
//	/debug/pprof/block              — where goroutines are WAITING (I/O, channels, syscalls)
//	/debug/pprof/mutex              — lock contention between goroutines
//	/debug/pprof/goroutine          — stack dump of all goroutines
//	/debug/pprof/trace?seconds=N    — full Go scheduler trace (syscall latency, GC events)
//
// WARNING: Only enable in non-production environments via DEBUG_PPROF_ENABLED=true.
// SetBlockProfileRate and SetMutexProfileFraction add ~2-5% CPU overhead.
func (s *Server) StartDebugServer(port int) error {
	// Capture every blocking operation (channel, I/O, syscall wait).
	// Rate=1 means sample every nanosecond of blocking — maximum precision.
	// Increase to 100+ if overhead is too high.
	runtime.SetBlockProfileRate(1)

	// Sample 1/5 of mutex contention events. Fraction=5 is a good balance
	// between overhead and signal quality for identifying hot locks.
	runtime.SetMutexProfileFraction(5)

	addr := fmt.Sprintf(":%d", port)
	s.logger.Info("Starting pprof debug server",
		"addr", addr,
		"block_profile_rate", 1,
		"mutex_profile_fraction", 5,
	)

	// DefaultServeMux already has pprof routes registered by the blank import above.
	return http.ListenAndServe(addr, nil)
}

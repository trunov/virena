package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"
)

func captureProfiles(profileDir string) {
	timestamp := time.Now().Format("20060102150405")

	cpuPath := filepath.Join(profileDir, "cpu"+timestamp+".prof")

	fcpu, err := os.Create(cpuPath)
	if err != nil {
		fmt.Println("CPU profile create err:", err)
		return
	}
	defer fcpu.Close()

	if err := pprof.StartCPUProfile(fcpu); err != nil {
		fmt.Println("CPU profile start err:", err)
		return
	}
	defer pprof.StopCPUProfile()

	time.Sleep(10 * time.Second)

	memPath := filepath.Join(profileDir, "mem"+timestamp+".prof")

	fmem, err := os.Create(memPath)
	if err != nil {
		fmt.Println("Memory profile create err:", err)
		return
	}
	defer fmem.Close()

	runtime.GC()
	if err := pprof.WriteHeapProfile(fmem); err != nil {
		fmt.Println("Memory profile write err:", err)
	}
}

func startPeriodicProfiling() *time.Ticker {
	ticker := time.NewTicker(2 * time.Minute)

	profileDir := os.Getenv("PROFILE_DIR")
	if profileDir == "" {
		profileDir = "profiles"
	}

	if err := os.MkdirAll(profileDir, 0755); err != nil {
		fmt.Println("Failed to create profile directory:", err)
	}

	go func() {
		for range ticker.C {
			captureProfiles(profileDir)
		}
	}()

	return ticker
}

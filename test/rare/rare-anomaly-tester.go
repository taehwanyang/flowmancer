package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

const (
	baselineRequests = 5
	baselineInterval = 1 * time.Minute

	burstDelayFromStart = 7 * time.Minute
	burstRequests       = 100

	burstRequestGap = 5 * time.Millisecond
)

func main() {
	transport := &http.Transport{
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}

	start := time.Now()

	log.Printf("phase 1: building rare baseline (%d requests, every %s)", baselineRequests, baselineInterval)
	baselineSuccess, baselineFailure := runBaselinePhase(client)

	log.Printf(
		"phase 1 complete: success=%d failure=%d elapsed=%s",
		baselineSuccess,
		baselineFailure,
		time.Since(start),
	)

	waitUntil(start.Add(burstDelayFromStart))

	log.Printf("phase 2: burst starts (%d sequential requests)", burstRequests)
	burstSuccess, burstFailure, burstElapsed := runBurstPhase(client)

	totalElapsed := time.Since(start)

	fmt.Println("==== RESULT ====")
	fmt.Println("baseline success:", baselineSuccess)
	fmt.Println("baseline failure:", baselineFailure)
	fmt.Println("burst success:", burstSuccess)
	fmt.Println("burst failure:", burstFailure)
	fmt.Println("burst elapsed:", burstElapsed)
	fmt.Println("total elapsed:", totalElapsed)

	if burstElapsed > 0 {
		fmt.Printf("burst RPS: %.2f\n", float64(burstRequests)/burstElapsed.Seconds())
	}
}

func runBaselinePhase(client *http.Client) (success int, failure int) {
	for i := 0; i < baselineRequests; i++ {
		log.Printf("baseline request #%d/%d", i+1, baselineRequests)

		if err := sendRequest(client); err != nil {
			failure++
			log.Printf("baseline request #%d FAILED: %v", i+1, err)
		} else {
			success++
			log.Printf("baseline request #%d SUCCEEDED", i+1)
		}

		if i < baselineRequests-1 {
			time.Sleep(baselineInterval)
		}
	}

	return success, failure
}

func runBurstPhase(client *http.Client) (success int, failure int, elapsed time.Duration) {
	start := time.Now()

	for i := 0; i < burstRequests; i++ {
		if err := sendRequest(client); err != nil {
			failure++
			log.Printf("burst request #%d FAILED: %v", i+1, err)
		} else {
			success++
		}

		if i < burstRequests-1 {
			time.Sleep(burstRequestGap)
		}
	}

	elapsed = time.Since(start)
	return success, failure, elapsed
}

func waitUntil(target time.Time) {
	if wait := time.Until(target); wait > 0 {
		log.Printf("waiting %s until burst phase", wait)
		time.Sleep(wait)
	}
}

func sendRequest(client *http.Client) error {
	tokenURL := "http://auth-test-authorization-server.auth.svc.cluster.local/oauth2/token"

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", "read")

	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return err
	}

	req.Close = true
	req.Header.Set("Connection", "close")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	basicAuth := base64.StdEncoding.EncodeToString([]byte("client:secret123"))
	req.Header.Set("Authorization", "Basic "+basicAuth)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("status: %d", resp.StatusCode)
}

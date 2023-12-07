package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type RoundRobinBalancer struct {
	activeUrls   []*url.URL
	removedUrls  map[*url.URL]struct{}
	currentIndex uint64
	lock         sync.Mutex
}

func NewRoundRobinBalancer(backendUrls []*url.URL) *RoundRobinBalancer {
	activeUrls := make([]*url.URL, 0, len(backendUrls))
	removedUrlsMap := make(map[*url.URL]struct{})

	for _, backendUrl := range backendUrls {
		parsedUrl, err := url.Parse(backendUrl.String())
		if err != nil {
			log.Printf("Invalid URL %s: %v", backendUrl, err)
			continue
		}
		activeUrls = append(activeUrls, parsedUrl)
	}

	return &RoundRobinBalancer{
		activeUrls:  activeUrls,
		removedUrls: removedUrlsMap,
	}
}

func (r *RoundRobinBalancer) GetNextURL() *url.URL {
	r.lock.Lock()
	defer r.lock.Unlock()

	if len(r.activeUrls) == 0 {
		return nil
	}

	index := atomic.AddUint64(&r.currentIndex, 1)
	return r.activeUrls[index%uint64(len(r.activeUrls))]
}

func (r *RoundRobinBalancer) RemoveURL(urlToRemove *url.URL) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.removedUrls[urlToRemove] = struct{}{}
	for i, u := range r.activeUrls {
		if u.String() == urlToRemove.String() {
			r.activeUrls = append(r.activeUrls[:i], r.activeUrls[i+1:]...)
			break
		}
	}
}

func (r *RoundRobinBalancer) CheckAndRestoreUrls() {
	r.lock.Lock()
	defer r.lock.Unlock()

	for urlStr := range r.removedUrls {
		_, err := http.Get(urlStr.String())
		if err == nil {
			parsedUrl, _ := url.Parse(urlStr.String())
			r.activeUrls = append(r.activeUrls, parsedUrl)
			delete(r.removedUrls, urlStr)
		}
	}
}

func sendRequest(balancer *RoundRobinBalancer) {
	serverURL := balancer.GetNextURL()
	if serverURL == nil {
		log.Println("No active servers available")
		return
	}

	resp, err := http.Get(serverURL.String())
	if err != nil {
		log.Printf("Error making request to %s: %v\n", serverURL, err)
		balancer.RemoveURL(serverURL)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("Server %s responded with non-200 status: %d\n", serverURL, resp.StatusCode)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response from %s: %v\n", serverURL, err)
		return
	}

	log.Printf("Response from server %s: %s\n", serverURL, body)
}

func main() {
	backends := []*url.URL{
		{
			Scheme: "http",
			Host:   "localhost:81",
		},
		{
			Scheme: "http",
			Host:   "localhost:82",
		},
		{
			Scheme: "http",
			Host:   "localhost:83",
		},
	}

	balancer := NewRoundRobinBalancer(backends)

	// Periodically check and restore removed servers
	go func() {
		for {
			time.Sleep(5 * time.Second)
			balancer.CheckAndRestoreUrls()
		}
	}()

	// Continuously send requests
	for {
		sendRequest(balancer)
		time.Sleep(1 * time.Second) // Throttle requests
	}
}

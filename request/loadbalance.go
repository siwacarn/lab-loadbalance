package request

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
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

	for _, backendsURL := range backendUrls {
		parsedURL, err := url.Parse(backendsURL.String())
		if err != nil {
			log.Printf("Invalid URL %s: %v", backendsURL, err)
			continue
		}
		activeUrls = append(activeUrls, parsedURL)
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

	// Use the current index value before incrementing it
	currentIndex := atomic.LoadUint64(&r.currentIndex)
	url := r.activeUrls[currentIndex%uint64(len(r.activeUrls))]
	atomic.AddUint64(&r.currentIndex, 1)

	return url
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

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, urlStr.String(), nil)
		if err != nil {
			log.Printf("Error creating request: %v\n", err)
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == 200 {
			parsedURL, err := url.Parse(urlStr.String())
			if err != nil {
				log.Printf("Invalid URL %s: %v", urlStr, err)
				continue
			}

			r.activeUrls = append(r.activeUrls, parsedURL)
			delete(r.removedUrls, urlStr)

			defer resp.Body.Close()
		}
	}
}

func SendRequest(balancer *RoundRobinBalancer) {
	for {
	next:
		serverURL := balancer.GetNextURL()
		if serverURL == nil {
			log.Println("No active servers available")
			return
		}

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, serverURL.String(), nil)
		if err != nil {
			log.Printf("Error creating request: %v\n", err)
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			// log.Printf("Error making request to %s: %v\n", serverURL, err)
			balancer.RemoveURL(serverURL)
			goto next
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Error reading response: %v\n", err)
				return
			}
			log.Printf("Response from server: %s\n", body)
			break
		}

		resp.Body.Close()
	}

}

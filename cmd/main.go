package main

import (
	"loadbalance/request"
	"net/url"
	"time"
)

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

	balancer := request.NewRoundRobinBalancer(backends)

	// Periodically check and restore removed servers
	go func() {
		for {
			time.Sleep(5 * time.Second)
			balancer.CheckAndRestoreUrls()
		}
	}()

	// Continuously send requests
	for {
		request.SendRequest(balancer)
		time.Sleep(1 * time.Second) // Throttle requests
	}
}

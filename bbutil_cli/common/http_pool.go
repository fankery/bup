package common

import (
	"net/http"
	"time"
)

var HttpClient = &http.Client{
	Timeout: time.Second * 10,
	Transport: &http.Transport{
		MaxIdleConnsPerHost: 1,
		MaxConnsPerHost:     2,
		IdleConnTimeout:     time.Second * 2,
	},
}

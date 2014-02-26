// +build cgo

package main

import (
	"net/http"
)

var getHttps = func(url string) (resp *http.Response, err error) {
	return http.Get(url)
}

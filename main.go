package main

// Taken mainly from the talk; "How I Write HTTP Web Services after Eight Years" by Mat Ryer
// 1. https://www.youtube.com/watch?v=rWBSMsLG8po
// 2. https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html

import (
	"net/http"
	"os"

	"github.com/komuw/goweb/server"
)

func main() {
	api := &myAPI{
		db:     "someDb",
		router: http.NewServeMux(),
	}

	err := server.Run(api, server.DefaultOpts())
	if err != nil {
		api.logger.Printf("\n server.Run error: %+v\n", err)
		os.Exit(1)
	}
}

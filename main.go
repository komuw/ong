package main

// Taken mainly from the talk; "How I Write HTTP Web Services after Eight Years" by Mat Ryer
// 1. https://www.youtube.com/watch?v=rWBSMsLG8po
// 2. https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html

import (
	"net/http"
	"os"

	"github.com/komuw/goweb/log"
	"github.com/komuw/goweb/middleware"

	"github.com/komuw/goweb/server"
)

func main() {
	api := NewMyApi("someDb")

	mux := server.NewMux([]server.MuxOpts{
		server.NewMuxOpts("api", server.MethodPost, api.handleAPI(), middleware.WithOpts("localhost")),
		server.NewMuxOpts("greeting", server.MethodGet, middleware.BasicAuth(api.handleGreeting(202), "user", "passwd"), middleware.WithOpts("localhost")),
		server.NewMuxOpts("serveDirectory", server.MethodAll, middleware.BasicAuth(api.handleFileServer(), "user", "passwd"), middleware.WithOpts("localhost")),
		server.NewMuxOpts("check", server.MethodGet, api.handleGreeting(200), middleware.WithOpts("localhost")),
	})

	err := server.Run(mux, server.DefaultOpts())
	if err != nil {
		mux.GetLogger().Error(err, log.F{
			"msg": "server.Run error",
		})
		os.Exit(1)
	}
}

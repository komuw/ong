package main

// Taken mainly from the talk; "How I Write HTTP Web Services after Eight Years" by Mat Ryer
// 1. https://www.youtube.com/watch?v=rWBSMsLG8po
// 2. https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html

import (
	"os"

	"github.com/komuw/goweb/log"
	"github.com/komuw/goweb/middleware"

	"github.com/komuw/goweb/server"
)

func main() {
	api := NewMyApi("someDb")

	mux := server.NewMux(server.Routes{
		server.NewRoute("/api", server.MethodPost, api.handleAPI(), middleware.WithOpts("localhost")),
		server.NewRoute("greeting", server.MethodGet, middleware.BasicAuth(api.handleGreeting(202), "user", "passwd"), middleware.WithOpts("localhost")),
		server.NewRoute("serveDirectory", server.MethodAll, middleware.BasicAuth(api.handleFileServer(), "user", "passwd"), middleware.WithOpts("localhost")),
		server.NewRoute("check/", server.MethodGet, api.handleGreeting(200), middleware.WithOpts("localhost")),
	})

	err := server.Run(mux, server.DefaultOpts())
	if err != nil {
		mux.GetLogger().Error(err, log.F{
			"msg": "server.Run error",
		})
		os.Exit(1)
	}
}

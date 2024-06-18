package main

import (
	"context"
	"os"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/mux"
	"github.com/komuw/ong/server"
)

func main() {
	l := log.New(context.Background(), os.Stdout, 100).With("pid", os.Getpid())
	const secretKey = "super-h@rd-Pas1word"

	api := NewApp(myDB{map[string]string{}}, l)

	basicAuth, err := middleware.BasicAuth(api.handleFileServer(), "user", "some-long-1passwd")
	if err != nil {
		panic(err)
	}

	mx := mux.New(
		config.WithOpts("localhost", 65081, secretKey, config.DirectIpStrategy, l),
		nil,
		mux.NewRoute(
			"/health",
			mux.MethodGet,
			api.health(secretKey),
		),
		mux.NewRoute(
			"staticAssets/:file",
			mux.MethodAll,
			basicAuth,
		),
		mux.NewRoute(
			"check/:age/",
			mux.MethodAll,
			api.check("world"),
		),
		mux.NewRoute(
			"login",
			mux.MethodAll,
			api.login(secretKey),
		),
		mux.NewRoute(
			"panic",
			mux.MethodAll,
			api.panic(),
		),
	)

	if e := server.Run(mx, config.DevOpts(l, secretKey)); e != nil {
		l.Error("server.Run error", "error", e)
		os.Exit(1)
	}
}

// myDB implements a dummy in-memory database.
type myDB struct{ m map[string]string }

func (m myDB) Get(key string) string { return "" }
func (m myDB) Set(key, value string) {}

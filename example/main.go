package main

import (
	"os"

	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/mux"
	"github.com/komuw/ong/server"
)

func main() {
	const secretKey = "super-h@rd-pa$$word"
	api := NewApp(myDB{map[string]string{}})
	l := log.New(os.Stdout, 1000)
	mux := mux.New(
		l,
		middleware.WithOpts("localhost", 65081, secretKey, middleware.DirectIpStrategy, l),
		nil,
		mux.NewRoute(
			"/health",
			mux.MethodGet,
			api.health(secretKey),
		),
		mux.NewRoute(
			"serveDirectory",
			mux.MethodAll,
			middleware.BasicAuth(api.handleFileServer(), "user", "some-long-passwd"),
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
	)

	err := server.Run(mux, server.DevOpts(l), l)
	if err != nil {
		l.Error(err, log.F{"msg": "server.Run error"})
		os.Exit(1)
	}
}

// myDB implements a dummy in-memory database.
type myDB struct{ m map[string]string }

func (m myDB) Get(key string) string { return "" }
func (m myDB) Set(key, value string) {}

package main

// Taken mainly from the talk; How I Write HTTP Web Services after Eight Years
// - https://www.youtube.com/watch?v=rWBSMsLG8po -  Mat Ryer.

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/sys/unix" // syscall package is deprecated
)

// myAPI rep component as struct
// shared dependencies as fields
// no global state
type myAPI struct {
	db     string
	router *http.ServeMux // some router
	logger *log.Logger    // some logger, maybe
}

// Make `myAPI` implement the http.Handler interface(https://golang.org/pkg/net/http/#Handler)
// use myAPI wherever you could use http.Handler(eg ListenAndServe)
func (s myAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Have one place for all routes.
// You can even move it to a routes.go file
func (s myAPI) routes() {
	s.router.HandleFunc("/api/",
		s.flocOptOut(s.handleAPI()),
	)
	s.router.HandleFunc("/greeting",
		// you can even have your handler take a `*template.Template` dependency
		s.flocOptOut(
			s.Auth(s.handleGreeting(202)),
		),
	)
	s.router.HandleFunc("/serveDirectory",
		s.flocOptOut(
			s.Auth(handleFileServer()),
		),
	)

	// etc
}

func handleFileServer() http.HandlerFunc {
	// Do NOT let `http.FileServer` be able to serve your root directory.
	// Otherwise, your .git folder and other sensitive info(including http://localhost:8080/main.go) may be available
	// instead create a folder that only has your templates and server that.
	fs := http.FileServer(http.Dir("./stuff"))
	realHandler := http.StripPrefix("somePrefix", fs).ServeHTTP
	return func(w http.ResponseWriter, req *http.Request) {
		log.Println(req.URL.Redacted())
		realHandler(w, req)
	}
}

// Handlers are methods on the server which gives them access to dependencies
// Remember, other handlers have access to `s` too, so be careful with data races
// Why return `http.HandlerFunc` instead of `http.Handler`?
// `HandlerFunc` implements `Handler` interface so they are kind of interchangeable
// Pick whichever is easier for you to use. Sometimes you might have to convert between them
func (s myAPI) handleAPI() http.HandlerFunc {
	// allows for handler specific setup
	thing := func() int {
		return 42
	}
	var once sync.Once
	var serverStart time.Time

	// return the handler
	return func(w http.ResponseWriter, r *http.Request) {
		// intialize somethings only once for perf
		once.Do(func() {
			s.logger.Println("called only once during the first request")
			serverStart = time.Now()
		})

		// use thing
		ting := thing()
		if ting != 42 {
			http.Error(w, "thing ought to be 42", http.StatusBadRequest)
			return
		}

		res := fmt.Sprintf("serverStart=%v\n. Hello. answer to life is %v \n", serverStart, ting)
		_, _ = w.Write([]byte(res))
	}
}

// you can take arguments for handler specific dependencies
func (s myAPI) handleGreeting(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// use code, which is a dependency specific to this handler
		w.WriteHeader(code)
	}
}

// TODO: add these security headers;
// https://web.dev/security-headers/

// flocOptOut disables floc which is otherwise ON by default
// see: https://github.com/WICG/floc#opting-out-of-computation
func (s myAPI) flocOptOut(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// code that is ran b4 wrapped handler
		w.Header().Set("Permissions-Policy", "interest-cohort=()")
		wrappedHandler(w, r)
	}
}

// middleware are just go functions
// you can run code before and/or after the wrapped hanlder
func (s myAPI) Auth(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	const realm = "enter username and password"
	return func(w http.ResponseWriter, r *http.Request) {
		// code that is ran b4 wrapped handler
		fmt.Println("code ran BEFORE wrapped handler")
		username, _, _ := r.BasicAuth()

		if username == "" { //|| pass == ""
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if subtle.ConstantTimeCompare([]byte(username), []byte("admin")) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		wrappedHandler(w, r)
		// you can also run code after wrapped handler here
		// you can even choose not to call wrapped handler at all
		fmt.Println("code ran AFTER wrapped handler")
	}
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	maxprocs.Set()

	ctx, cancel := context.WithCancel(context.Background())

	// TODO: does the server have to be a pointer?
	api := myAPI{
		db:     "someDb",
		router: http.NewServeMux(),
		logger: log.New(os.Stdout, "logger: ", log.Lshortfile),
	}
	api.routes()

	serverPort := ":8080"
	network := "tcp"
	address := fmt.Sprintf("127.0.0.1%s", serverPort)
	server := &http.Server{
		Addr: serverPort,

		// 1. https://blog.simon-frey.eu/go-as-in-golang-standard-net-http-config-will-break-your-production
		// 2. https://blog.cloudflare.com/exposing-go-on-the-internet/
		// 3. https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
		// 4. https://github.com/golang/go/issues/27375
		Handler:           http.TimeoutHandler(api, 10*time.Second, "Custom Server timeout"),
		ReadHeaderTimeout: 1 * time.Second,
		ReadTimeout:       1 * time.Second,
		WriteTimeout:      1 * time.Second,
		IdleTimeout:       120 * time.Second,

		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	sigHandler(server, ctx, cancel)

	api.logger.Printf("server listening at %s", address)
	return serve(server, network, address, ctx)
}

func sigHandler(srv *http.Server, ctx context.Context, cancel context.CancelFunc) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGQUIT, unix.SIGHUP)
	go func() {
		<-sigs
		cancel()
		_ = srv.Shutdown(ctx)
	}()
}

func serve(srv *http.Server, network string, address string, ctx context.Context) error {
	cfg := &net.ListenConfig{Control: func(network, address string, conn syscall.RawConn) error {
		return conn.Control(func(descriptor uintptr) {
			_ = unix.SetsockoptInt(
				int(descriptor),
				unix.SOL_SOCKET,
				// go vet will complain if we used syscall.SO_REUSEPORT, even though it would work.
				// this is because Go considers syscall pkg to be frozen. The same goes for syscall.SetsockoptInt
				// so we use x/sys/unix
				// see: https://github.com/golang/go/issues/26771
				unix.SO_REUSEPORT,
				1,
			)
			_ = unix.SetsockoptInt(
				int(descriptor),
				unix.SOL_SOCKET,
				unix.SO_REUSEADDR,
				1,
			)
		})
	}}
	l, err := cfg.Listen(ctx, network, address)
	if err != nil {
		return err
	}

	return srv.Serve(l)
}

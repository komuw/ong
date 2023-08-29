package mux

import (
	"bufio"
	"bytes"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Most of the code here is inspired(or taken from) by:
//   (a) https://github.com/golang/go/blob/go1.21.0/src/net/http/pprof/pprof.go whose license(BSD 3-Clause "New") can be found here: https://github.com/golang/go/blob/go1.21.0/LICENSE
//

// See documentation:
// https://github.com/golang/go/blob/go1.21.0/src/net/http/pprof/pprof.go#L5-L70

// TODO:
// func init() {
// 	http.HandleFunc("/debug/pprof/", Index)
// 	http.HandleFunc("/debug/pprof/cmdline", Cmdline)
// 	http.HandleFunc("/debug/pprof/profile", Profile)
// 	http.HandleFunc("/debug/pprof/symbol", Symbol)
// 	http.HandleFunc("/debug/pprof/trace", Trace)
// }

// TODO: un-export API.

// TODO: borrow tests.

// TODO: move to middleware??

const (
	/*
		The pprof tool supports fetching profles by duration.
		eg; fetch cpu profile for the last 5mins(300sec):
			go tool pprof http://localhost:65079/debug/pprof/profile?seconds=300
		This may fail with an error like:
			http://localhost:65079/debug/pprof/profile?seconds=300: server response: 400 Bad Request - profile duration exceeds server's WriteTimeout
		So we need to be generous with our timeouts. Which is okay since pprof runs in a mux that is not exposed to the internet(localhost)
	*/
	readTimeout  = 30 * time.Second
	writeTimeout = 30 * time.Minute
)

// Cmdline responds with the running program's
// command line, with arguments separated by NUL bytes.
// The package initialization registers it as /debug/pprof/cmdline.
func Cmdline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, strings.Join(os.Args, "\x00"))
}

func sleep(r *http.Request, d time.Duration) {
	select {
	case <-time.After(d):
	case <-r.Context().Done():
	}
}

// TODO: only call it once.
func extendTimeouts(w http.ResponseWriter) {
	now := time.Now()
	rc := http.NewResponseController(w)

	if err := rc.SetReadDeadline(now.Add(readTimeout)); err != nil {
		err := fmt.Errorf("ong/mux: unable to extend pprof readTimeout: %w", err)
		serveError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := rc.SetWriteDeadline(now.Add(writeTimeout)); err != nil {
		err := fmt.Errorf("ong/mux: unable to extend pprof writeTimeout: %w", err)
		serveError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func durationExceedsWriteTimeout(r *http.Request, seconds float64) bool {
	// TODO: maybe kill this func?
	return seconds >= writeTimeout.Seconds()
}

func serveError(w http.ResponseWriter, status int, txt string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Go-Pprof", "1")
	w.Header().Del("Content-Disposition")
	w.WriteHeader(status)
	fmt.Fprintln(w, txt)
}

// Profile responds with the pprof-formatted cpu profile.
// Profiling lasts for duration specified in seconds GET parameter, or for 30 seconds if not specified.
// The package initialization registers it as /debug/pprof/profile.
func Profile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	sec, err := strconv.ParseInt(r.FormValue("seconds"), 10, 64)
	if sec <= 0 || err != nil {
		sec = 30
	}

	if durationExceedsWriteTimeout(r, float64(sec)) {
		serveError(w, http.StatusBadRequest, "profile duration exceeds server's WriteTimeout")
		return
	}

	// Set Content Type assuming StartCPUProfile will work,
	// because if it does it starts writing.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="profile"`)
	if err := pprof.StartCPUProfile(w); err != nil {
		// StartCPUProfile failed, so no writes yet.
		serveError(w, http.StatusInternalServerError,
			fmt.Sprintf("Could not enable CPU profiling: %s", err))
		return
	}
	sleep(r, time.Duration(sec)*time.Second)
	pprof.StopCPUProfile()
}

// Trace responds with the execution trace in binary form.
// Tracing lasts for duration specified in seconds GET parameter, or for 1 second if not specified.
// The package initialization registers it as /debug/pprof/trace.
func Trace(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	sec, err := strconv.ParseFloat(r.FormValue("seconds"), 64)
	if sec <= 0 || err != nil {
		sec = 1
	}

	if durationExceedsWriteTimeout(r, sec) {
		serveError(w, http.StatusBadRequest, "profile duration exceeds server's WriteTimeout")
		return
	}

	// Set Content Type assuming trace.Start will work,
	// because if it does it starts writing.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="trace"`)
	if err := trace.Start(w); err != nil {
		// trace.Start failed, so no writes yet.
		serveError(w, http.StatusInternalServerError,
			fmt.Sprintf("Could not enable tracing: %s", err))
		return
	}
	sleep(r, time.Duration(sec*float64(time.Second)))
	trace.Stop()
}

// Symbol looks up the program counters listed in the request,
// responding with a table mapping program counters to function names.
// The package initialization registers it as /debug/pprof/symbol.
func Symbol(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// We have to read the whole POST body before
	// writing any output. Buffer the output here.
	var buf bytes.Buffer

	// We don't know how many symbols we have, but we
	// do have symbol information. Pprof only cares whether
	// this number is 0 (no symbols available) or > 0.
	fmt.Fprintf(&buf, "num_symbols: 1\n")

	var b *bufio.Reader
	if r.Method == "POST" {
		b = bufio.NewReader(r.Body)
	} else {
		b = bufio.NewReader(strings.NewReader(r.URL.RawQuery))
	}

	for {
		word, err := b.ReadSlice('+')
		if err == nil {
			word = word[0 : len(word)-1] // trim +
		}
		pc, _ := strconv.ParseUint(string(word), 0, 64)
		if pc != 0 {
			f := runtime.FuncForPC(uintptr(pc))
			if f != nil {
				fmt.Fprintf(&buf, "%#x %s\n", pc, f.Name())
			}
		}

		// Wait until here to check for err; the last
		// symbol will have an err because it doesn't end in +.
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(&buf, "reading request: %v\n", err)
			}
			break
		}
	}

	w.Write(buf.Bytes())
}

// TODO:
// Handler returns an HTTP handler that serves the named profile.
// // Available profiles can be found in [runtime/pprof.Profile].
// func Handler(name string) http.Handler {
// 	return pprofHandler(name)
// }

type pprofHandler string

func (name pprofHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	p := pprof.Lookup(string(name))
	if p == nil {
		serveError(w, http.StatusNotFound, "Unknown profile")
		return
	}
	if sec := r.FormValue("seconds"); sec != "" {
		// name.serveDeltaProfile(w, r, p, sec)
		// TODO:
		err := fmt.Errorf("TODO: ong/mux: handle serveDeltaProfile. name=%s, seconds=%s", name, sec)
		serveError(w, http.StatusInternalServerError, err.Error())
		return
	}
	gc, _ := strconv.Atoi(r.FormValue("gc"))
	if name == "heap" && gc > 0 {
		runtime.GC()
	}
	debug, _ := strconv.Atoi(r.FormValue("debug"))
	if debug != 0 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	}
	p.WriteTo(w, debug)
}

// func (name pprofHandler) serveDeltaProfile(w http.ResponseWriter, r *http.Request, p *pprof.Profile, secStr string) {
// 	sec, err := strconv.ParseInt(secStr, 10, 64)
// 	if err != nil || sec <= 0 {
// 		serveError(w, http.StatusBadRequest, `invalid value for "seconds" - must be a positive integer`)
// 		return
// 	}
// 	if !profileSupportsDelta[name] {
// 		serveError(w, http.StatusBadRequest, `"seconds" parameter is not supported for this profile type`)
// 		return
// 	}
// 	// 'name' should be a key in profileSupportsDelta.
// 	if durationExceedsWriteTimeout(r, float64(sec)) {
// 		serveError(w, http.StatusBadRequest, "profile duration exceeds server's WriteTimeout")
// 		return
// 	}
// 	debug, _ := strconv.Atoi(r.FormValue("debug"))
// 	if debug != 0 {
// 		serveError(w, http.StatusBadRequest, "seconds and debug params are incompatible")
// 		return
// 	}
// 	p0, err := collectProfile(p)
// 	if err != nil {
// 		serveError(w, http.StatusInternalServerError, "failed to collect profile")
// 		return
// 	}

// 	t := time.NewTimer(time.Duration(sec) * time.Second)
// 	defer t.Stop()

// 	select {
// 	case <-r.Context().Done():
// 		err := r.Context().Err()
// 		if err == context.DeadlineExceeded {
// 			serveError(w, http.StatusRequestTimeout, err.Error())
// 		} else { // TODO: what's a good status code for canceled requests? 400?
// 			serveError(w, http.StatusInternalServerError, err.Error())
// 		}
// 		return
// 	case <-t.C:
// 	}

// 	p1, err := collectProfile(p)
// 	if err != nil {
// 		serveError(w, http.StatusInternalServerError, "failed to collect profile")
// 		return
// 	}
// 	ts := p1.TimeNanos
// 	dur := p1.TimeNanos - p0.TimeNanos

// 	p0.Scale(-1)

// 	p1, err = profile.Merge([]*profile.Profile{p0, p1})
// 	if err != nil {
// 		serveError(w, http.StatusInternalServerError, "failed to compute delta")
// 		return
// 	}

// 	p1.TimeNanos = ts // set since we don't know what profile.Merge set for TimeNanos.
// 	p1.DurationNanos = dur

// 	w.Header().Set("Content-Type", "application/octet-stream")
// 	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-delta"`, name))
// 	p1.Write(w)
// }

// func collectProfile(p *pprof.Profile) (*profile.Profile, error) {
// 	var buf bytes.Buffer
// 	if err := p.WriteTo(&buf, 0); err != nil {
// 		return nil, err
// 	}
// 	ts := time.Now().UnixNano()
// 	p0, err := profile.Parse(&buf)
// 	if err != nil {
// 		return nil, err
// 	}
// 	p0.TimeNanos = ts
// 	return p0, nil
// }

var profileSupportsDelta = map[pprofHandler]bool{
	"allocs":       true,
	"block":        true,
	"goroutine":    true,
	"heap":         true,
	"mutex":        true,
	"threadcreate": true,
}

var profileDescriptions = map[string]string{
	"allocs":       "A sampling of all past memory allocations",
	"block":        "Stack traces that led to blocking on synchronization primitives",
	"cmdline":      "The command line invocation of the current program",
	"goroutine":    "Stack traces of all current goroutines. Use debug=2 as a query parameter to export in the same format as an unrecovered panic.",
	"heap":         "A sampling of memory allocations of live objects. You can specify the gc GET parameter to run GC before taking the heap sample.",
	"mutex":        "Stack traces of holders of contended mutexes",
	"profile":      "CPU profile. You can specify the duration in the seconds GET parameter. After you get the profile file, use the go tool pprof command to investigate the profile.",
	"threadcreate": "Stack traces that led to the creation of new OS threads",
	"trace":        "A trace of execution of the current program. You can specify the duration in the seconds GET parameter. After you get the trace file, use the go tool trace command to investigate the trace.",
}

type profileEntry struct {
	Name  string
	Href  string
	Desc  string
	Count int
}

// Index responds with the pprof-formatted profile named by the request.
// For example, "/debug/pprof/heap" serves the "heap" profile.
// Index responds to a request for "/debug/pprof/" with an HTML page
// listing the available profiles.
func Index(w http.ResponseWriter, r *http.Request) {
	if name, found := strings.CutPrefix(r.URL.Path, "/debug/pprof/"); found {
		if name != "" {
			pprofHandler(name).ServeHTTP(w, r)
			return
		}
	}

	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var profiles []profileEntry
	for _, p := range pprof.Profiles() {
		profiles = append(profiles, profileEntry{
			Name:  p.Name(),
			Href:  fmt.Sprintf("pprof/%s", p.Name()),
			Desc:  profileDescriptions[p.Name()],
			Count: p.Count(),
		})
	}

	// Adding other profiles exposed from within this package
	for _, p := range []string{"cmdline", "profile", "trace"} {
		profiles = append(profiles, profileEntry{
			Name: p,
			Href: fmt.Sprintf("pprof/%s", p),
			Desc: profileDescriptions[p],
		})
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	if err := indexTmplExecute(w, profiles); err != nil {
		log.Print(err)
	}
}

func indexTmplExecute(w io.Writer, profiles []profileEntry) error {
	var b bytes.Buffer
	b.WriteString(`<html>
<head>
<title>/debug/pprof/</title>
<style>
.profile-name{
	display:inline-block;
	width:6rem;
}
</style>
</head>
<body>
/debug/pprof/
<br>
<p>Set debug=1 as a query parameter to export in legacy text format</p>
<br>
Types of profiles available:
<table>
<thead><td>Count</td><td>Profile</td></thead>
`)

	for _, profile := range profiles {
		link := &url.URL{Path: profile.Href, RawQuery: "debug=1"}
		fmt.Fprintf(&b, "<tr><td>%d</td><td><a href='%s'>%s</a></td></tr>\n", profile.Count, link, html.EscapeString(profile.Name))
	}

	b.WriteString(`</table>
<a href="goroutine?debug=2">full goroutine stack dump</a>
<br>
<p>
Profile Descriptions:
<ul>
`)
	for _, profile := range profiles {
		fmt.Fprintf(&b, "<li><div class=profile-name>%s: </div> %s</li>\n", html.EscapeString(profile.Name), html.EscapeString(profile.Desc))
	}
	b.WriteString(`</ul>
</p>
</body>
</html>`)

	_, err := w.Write(b.Bytes())
	return err
}

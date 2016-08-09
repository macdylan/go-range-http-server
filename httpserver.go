// 快速提供 HTTP 服务的小工具
//
// > ./go-range-http-server -l :8888 -p /path/to/file
// > ./go-range-http-server -l :8888 -p /path/to/dir
// 2016/08/09 19:30:00 Serving HTTP on 0.0.0.0:8888, dir: "dir/"
// 2016/08/09 19:30:01 [1] >>> GET - 127.0.0.1:51560 - /
// 2016/08/09 19:30:01 [1] <<< 0 1.192874ms 7bytes
// 2016/08/09 19:30:01 [2] >>> GET - 127.0.0.1:51561 - /favicon.ico
// 2016/08/09 19:30:01 [2] <<< 404 60.777µs 19bytes
//
//
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"sync/atomic"
	"time"
)

const VERSION = "go-range-http-server/1.0"

var (
	connIdx int32 = 0
	_listen       = flag.String("l", "0.0.0.0:2016", "")
	_public       = flag.String("p", ".", "file or directory")
	// _index  = flag.Bool("i", true, "enable or disable the directory listing output")
)

type Server interface {
	Serve(http.ResponseWriter, *http.Request)
}

type MultiFileServer struct {
	h http.Handler
}
type SingleFileServer struct {
	fullPath string
}

func (s *MultiFileServer) Serve(w http.ResponseWriter, r *http.Request) {
	s.h.ServeHTTP(w, r)
}

func (s *SingleFileServer) Serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path.Base(s.fullPath)))
	http.ServeFile(w, r, s.fullPath)
}

func NewMultiFileServer(path string) *MultiFileServer {
	return &MultiFileServer{
		h: http.FileServer(http.Dir(path)),
	}
}

func NewSingleFileServer(path string) *SingleFileServer {
	return &SingleFileServer{
		fullPath: path,
	}
}

type ResponseWriter struct {
	http.ResponseWriter
	Status int
	Length int
}

func (w *ResponseWriter) WriteHeader(status int) {
	w.Status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *ResponseWriter) Write(p []byte) (int, error) {
	l, e := w.ResponseWriter.Write(p)
	w.Length = l
	return l, e
}

type Handler struct {
	Server Server
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	idx := atomic.AddInt32(&connIdx, 1)
	log.Printf("[%d] >>> %s - %s - %s", idx, r.Method, r.RemoteAddr, r.RequestURI)

	wr := &ResponseWriter{
		ResponseWriter: w,
	}
	wr.Header().Add("Server", VERSION)

	startTime := time.Now()
	h.Server.Serve(wr, r)
	timeDur := time.Since(startTime)

	log.Printf("[%d] <<< %d %s %dbytes", idx, wr.Status, timeDur.String(), wr.Length)
}

func main() {
	var handler *Handler

	flag.Parse()

	fd, err := os.Open(*_public)
	if err != nil {
		log.Fatal(err)
	}

	st, err := fd.Stat()
	switch {
	case st.IsDir():
		log.Printf("Serving HTTP on %s, dir: \"%s\"", *_listen, *_public)
		handler = &Handler{NewMultiFileServer(*_public)}
	default:
		log.Printf("Serving HTTP on %s, file: \"%s\", %dbytes", *_listen, st.Name(), st.Size())
		handler = &Handler{NewSingleFileServer(*_public)}
	}

	log.Fatal(http.ListenAndServe(*_listen, handler))
}

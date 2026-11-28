// A WebDAV server for Carryover's PC <-> Android end-to-end test: serves a
// folder without authentication and logs each request.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"golang.org/x/net/webdav"
)

func main() {
	dir := flag.String("dir", ".", "folder to serve")
	addr := flag.String("addr", "127.0.0.1:8765", "listen address")
	flag.Parse()
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		log.Fatal(err)
	}
	h := &webdav.Handler{
		FileSystem: webdav.Dir(*dir),
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil {
				log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
			} else {
				log.Printf("%s %s", r.Method, r.URL.Path)
			}
		},
	}
	log.Printf("serving %s on http://%s", *dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, h))
}

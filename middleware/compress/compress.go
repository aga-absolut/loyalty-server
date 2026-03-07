package compress

import (
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"strings"
)

type gzipWriter struct {
	http.ResponseWriter
	zipWriter io.Writer
}

func (r gzipWriter) Write(bytes []byte) (int, error) {
	return r.zipWriter.Write(bytes)
}

func Compress(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, r)
			return
		}
		newWriter := gzip.NewWriter(w)
		defer newWriter.Close()

		w.Header().Set("Content-Encoding", "gzip")
		h.ServeHTTP(gzipWriter{ResponseWriter: w, zipWriter: newWriter}, r)
	})
}

func Decompress(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") != "gzip" {
			h.ServeHTTP(w, r)
			return
		}
		zipReader, err := gzip.NewReader(r.Body)
		if err != nil {
			log.Fatal("error", err)
			return
		}
		r.Body = zipReader
		defer r.Body.Close()
		h.ServeHTTP(w, r)
	})
}

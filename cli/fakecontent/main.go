package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"log"
	mrand "math/rand"
	"net/http"
	"sync"
	"time"
)

func main() {
	var buf [100 * 1024]byte
	rand.Reader.Read(buf[:])

	offset := mrand.Int31n(100 * 1024)

	var l sync.Mutex
	var content []byte
	var etag string

	go func() {
		for {
			var ret bytes.Buffer
			sha := sha1.New()
			mw := io.MultiWriter(&ret, sha)

			mw.Write(buf[:offset])
			io.CopyN(mw, rand.Reader, 8)
			mw.Write(buf[offset:])

			l.Lock()
			content = ret.Bytes()
			etag = hex.EncodeToString(sha.Sum(nil))
			l.Unlock()

			log.Println("Updated content")
			<-time.After(60 * time.Second)
		}
	}()

	log.Printf("Serving 100k+8 bytes of random content with bytes %d to %d different on each request\n", offset, offset+8)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", etag)
		w.Header().Set("Cache-Control", "max-age=60")

		l.Lock()
		defer l.Unlock()
		http.ServeContent(w, r, "", time.Now(), bytes.NewReader(content))
	})
	http.ListenAndServe(":8181", nil)
}

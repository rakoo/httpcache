package httpcache

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
)

type diffCache struct {
	innerCache Cache
}

func NewDiffCache(c Cache) Cache {
	return diffCache{c}
}

func (dc diffCache) Header(key string) (Header, error) {
	return dc.innerCache.Header(key)
}

func (dc diffCache) Store(res *Resource, keys ...string) error {
	// TODO: check if diffable
	oldEtag := res.Header().Get("Etag")
	if oldEtag != "" {
		for _, key := range keys {
			oldResource, err := dc.innerCache.Retrieve(key)
			if err == nil {
				diff := makeDiff(oldResource, res)
				diffKey := "vcdiff-" + oldEtag + "-" + key
				log.Println("Storing under", diffKey)
				if err = dc.innerCache.Store(diff, diffKey); err != nil {
					return err
				}
			}
		}

	}
	return dc.innerCache.Store(res, keys...)
}

func (dc diffCache) Retrieve(key string) (*Resource, error) {
	return dc.innerCache.Retrieve(key)
}

func (dc diffCache) Invalidate(keys ...string) {
	// TODO: understand what this does
	dc.innerCache.Invalidate(keys...)
}

func (dc diffCache) Freshen(res *Resource, keys ...string) error {
	// TODO: understand what this does
	return dc.innerCache.Freshen(res, keys...)
}

func makeDiff(oldR, newR *Resource) *Resource {
	// Store prev offset to reset it later
	prevOldROffset, err := oldR.Seek(0, os.SEEK_CUR)
	if err != nil {
		return nil
	}

	oldR.Seek(0, os.SEEK_SET)
	defer oldR.Seek(prevOldROffset, os.SEEK_SET)

	buf, err := ioutil.ReadAll(oldR.ReadSeekCloser)
	if err != nil {
		return nil
	}

	// Create tmp file for vcdiff dict
	tmpFile, err := ioutil.TempFile("", "dict-")
	if err != nil {
		return nil
	}
	io.Copy(tmpFile, bytes.NewReader(buf))
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Store prev offset to reset it later
	prevNewROffset, err := newR.Seek(0, os.SEEK_CUR)
	if err != nil {
		return nil
	}
	defer newR.Seek(prevNewROffset, os.SEEK_SET)

	cmd := exec.Command("vcdiff", "delta", "-dictionary", tmpFile.Name(), "-interleaved", "-checksum")
	cmd.Stdin = newR.ReadSeekCloser

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}

	resHeader := make(http.Header, len(oldR.Header()))
	for k, vv := range oldR.Header() {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		resHeader[k] = vv2
	}
	res := NewResourceBytes(226, out.Bytes(), resHeader)
	res.Header().Set("Content-Length", strconv.Itoa(out.Len()))
	res.Header().Set("IM", "vcdiff")
	return res
}

package views

import (
	"bytes"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rafaelmartins/distfiles/internal/settings"
	"github.com/rafaelmartins/distfiles/internal/tarfile"
)

var (
	reSha512 = regexp.MustCompile(`^([0-9a-f]{128}) (\*| )(.+)$`)
)

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	fmt.Fprintln(w, "OK")
}

func report(w http.ResponseWriter, code int, tag string, err error) {
	if err != nil {
		log.Printf("error: %s", err.Error())
	}
	http.Error(w, tag, code)
}

func Upload(w http.ResponseWriter, r *http.Request) {
	s, err := settings.Get()
	if err != nil {
		report(w, 500, "SETTINGS", err)
		return
	}

	u, _, ok := r.BasicAuth()
	if !ok {
		report(w, 401, "NOAUTH", nil)
		return
	}

	if subtle.ConstantTimeCompare([]byte(u), []byte(s.AuthToken)) == 0 {
		report(w, 401, "BADAUTH", nil)
		return
	}

	if err := r.ParseMultipartForm(32 * 1024 * 1024); err != nil {
		report(w, 400, "BADFORM_FILE", err)
		return
	}

	project, ok := r.Form["project"]
	if !ok || len(project) != 1 {
		report(w, 400, "BADFORM", nil)
		return
	}
	pn := project[0]

	version, ok := r.Form["version"]
	if !ok || len(version) != 1 {
		report(w, 400, "BADFORM", nil)
		return
	}
	pv := version[0]

	p := pn + "-" + pv

	sha512Form, ok := r.Form["sha512"]
	if !ok || len(sha512Form) != 1 {
		report(w, 400, "BADFORM", nil)
		return
	}
	sha512Matches := reSha512.FindStringSubmatch(sha512Form[0])
	if len(sha512Matches) != 4 {
		report(w, 400, "BADFORM_SHA512", nil)
		return
	}
	filehash := sha512Matches[1]
	filename := sha512Matches[3]
	if len(filename) < 4 {
		report(w, 400, "BADFORM_FILENAME_LENGTH", nil)
		return
	}
	if strings.ContainsAny(filename, "/\\") {
		report(w, 400, "BADFORM_FILENAME_SLASH", nil)
		return
	}

	f, fh, err := r.FormFile("file")
	if err != nil {
		if err == http.ErrMissingFile {
			report(w, 400, "BADFORM_NOFILE", err)
			return
		}
		report(w, 400, "BADFORM_FILE", err)
		return
	}
	defer f.Close()

	if fh.Filename != filename {
		report(w, 400, "BADFORM_SHA512_FILENAME", nil)
		return
	}

	sum, err := hex.DecodeString(filehash)
	if err != nil {
		report(w, 400, "BADFORM_SHA512", err)
		return
	}

	sha := sha512.New()
	if _, err := io.Copy(sha, f); err != nil {
		report(w, 500, "SHA512_SUM", err)
		return
	}

	if bytes.Compare(sha.Sum(nil), sum) != 0 {
		report(w, 400, "BADFORM_SHA512_HASH", err)
		return
	}

	destdir := filepath.Join(s.StorageDir, pn, p)
	if err := os.MkdirAll(destdir, 0777); err != nil {
		report(w, 500, "DIRECTORY_CREATE", err)
		return
	}

	if err := ioutil.WriteFile(filepath.Join(destdir, filename+".sha512"), []byte(sha512Form[0]), 0666); err != nil {
		report(w, 500, "SHA512_FILE", err)
		return
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		report(w, 500, "SEEK", err)
		return
	}
	fp, err := os.Create(filepath.Join(destdir, filename))
	if err != nil {
		report(w, 500, "FILE_CREATE", err)
		return
	}
	_, err = io.Copy(fp, f)
	if ferr := fp.Close(); ferr != nil {
		report(w, 500, "FILE_CLOSE", ferr)
		return
	}
	if err != nil {
		report(w, 500, "FILE_COPY", err)
		return
	}

	latest := filepath.Join(s.StorageDir, pn, "LATEST")
	if _, err := os.Lstat(latest); err == nil {
		if err := os.Remove(latest); err != nil {
			report(w, 500, "LATEST_REMOVE", err)
			return
		}
	}

	if err := os.Symlink(p, latest); err != nil {
		report(w, 500, "LATEST", err)
		return
	}

	if release := r.Form["release"]; len(release) == 1 && (release[0] == "1" || release[0] == "true") {
		latest_release := filepath.Join(s.StorageDir, pn, "LATEST_RELEASE")
		if _, err := os.Lstat(latest_release); err == nil {
			if err := os.Remove(latest_release); err != nil {
				report(w, 500, "LATEST_RELEASE_REMOVE", err)
				return
			}
		}

		if err := os.Symlink(p, latest_release); err != nil {
			report(w, 500, "LATEST_RELEASE", err)
			return
		}
	}

	if extract := r.Form["extract"]; len(extract) == 1 && (extract[0] == "1" || extract[0] == "true") {
		if err := tarfile.Untar(destdir, f); err != nil {
			report(w, 500, "EXTRACT", err)
			return
		}
	}

	Health(w, r)
}

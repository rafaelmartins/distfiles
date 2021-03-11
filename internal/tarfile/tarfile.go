package tarfile

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

func Untar(path string, rs io.ReadSeeker) error {
	// poor man's format detection
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return err
	}

	mn := make([]byte, 5)
	if _, err := rs.Read(mn); err != nil {
		return err
	}

	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return err
	}

	var (
		r   io.Reader
		err error
	)
	if strings.HasPrefix(string(mn), "\x1f\x8b\x08") {
		r, err = gzip.NewReader(rs)
	} else if strings.HasPrefix(string(mn), "BZh") {
		r = bzip2.NewReader(rs)
	} else if strings.HasPrefix(string(mn), "\x5d\x00\x00\x80") || strings.HasPrefix(string(mn), "\xfd7zXZ") {
		r, err = xz.NewReader(rs)
	} else {
		return errors.New("tarfile: unknown compression format")
	}
	if err != nil {
		return err
	}

	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// this is the best we can do without running as root :)

		if strings.Contains(hdr.Name, "..") {
			return fmt.Errorf("tarfile: invalid filename: %s", hdr.Name)
		}
		fn := filepath.Join(path, hdr.Name)
		m := os.FileMode(hdr.Mode) & os.ModePerm

		// cleanup whatever we had previously
		if st, err := os.Stat(fn); err == nil {
			if st.IsDir() {
				// as soon as we found an existing dir, cleanup everything.
				// this saves us from cleaning up each file or link afterwards and gives us a clean base to extract
				if err := os.RemoveAll(fn); err != nil {
					return err
				}
			} else if err := os.Remove(fn); err != nil {
				return err
			}
		}

		// FIXME: change link mtimes somehow
		switch hdr.Typeflag {
		case tar.TypeLink:
			err = os.Link(hdr.Linkname, fn)
		case tar.TypeSymlink:
			err = os.Symlink(hdr.Linkname, fn)
		case tar.TypeDir:
			err = os.Mkdir(fn, m)
			// the ultimate trick (tm)
			defer os.Chtimes(fn, time.Now(), hdr.ModTime)
		case tar.TypeReg:
			data, ferr := ioutil.ReadAll(tr)
			if ferr != nil {
				return ferr
			}
			if ferr := ioutil.WriteFile(fn, data, m); ferr != nil {
				return ferr
			}
			err = os.Chtimes(fn, time.Now(), hdr.ModTime)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

package settings

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

var (
	settings *Settings
)

type Settings struct {
	AuthRealm  string
	AuthToken  string
	ListenAddr string
	StorageDir string
}

func getString(key string, def string, required bool) (string, error) {
	if v, found := os.LookupEnv(key); found {
		if required && v == "" {
			return "", fmt.Errorf("settings: %s empty", key)
		}
		return v, nil
	}
	if required && def == "" {
		return "", fmt.Errorf("settings: %s missing", key)
	}
	return def, nil
}

func getUint(key string, def uint64, required bool, base int, bitSize int) (uint64, error) {
	v, err := getString(key, strconv.FormatUint(def, base), required)
	if err != nil {
		return 0, err
	}
	v2, err := strconv.ParseUint(v, base, bitSize)
	if err != nil {
		return 0, err
	}
	if required && v2 == 0 {
		return 0, fmt.Errorf("settings: %s empty", key)
	}
	return v2, nil
}

func Get() (*Settings, error) {
	if settings != nil {
		return settings, nil
	}

	var err error
	s := &Settings{}

	s.AuthRealm, err = getString("DISTFILES_AUTH_REALM", "distfiles", true)
	if err != nil {
		return nil, err
	}

	s.AuthToken, err = getString("DISTFILES_AUTH_TOKEN", "", true)
	if err != nil {
		return nil, err
	}

	s.ListenAddr, err = getString("DISTFILES_LISTEN_ADDR", ":8000", true)
	if err != nil {
		return nil, err
	}

	s.StorageDir, err = getString("DISTFILES_STORAGE_DIR", "data", true)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(s.StorageDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.MkdirAll(s.StorageDir, 0777); err != nil {
			return nil, err
		}
	} else if !st.IsDir() {
		return nil, errors.New("DISTFILES_STORAGE_DIR is not a directory")
	}

	settings = s

	return s, nil
}

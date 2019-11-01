package annote

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var (
	FileCacheRoot string
	TimeoutClient *http.Client
	ErrDownload   = errors.New("Error copying data from Curate")
)

// FindFileInCache will see if the given base name is in the cache.
// It it is, an absolute path to the file is returned. Otherwise an
// empty string is returned.
func FindFileInCache(base string) string {
	path := resolveCache(base)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return ""
	}
	return path
}

// resolveCache takes a base name and returns the path location
// the file should be at if it is in the cache.
func resolveCache(base string) string {
	if FileCacheRoot == "" {
		var err error
		FileCacheRoot, err = filepath.Abs("download_cache")
		if err != nil {
			log.Println("resolving FileCacheRoot:", err)
			return ""
		}
	}
	return filepath.Join(FileCacheRoot, base)
}

func DownloadFileToCache(base string, url string) error {
	if TimeoutClient == nil {
		TimeoutClient = &http.Client{
			Timeout: 60 * time.Minute,
		}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	// put "bot" in the user agent so it won't be counted in the curate metrics
	req.Header.Set("User-Agent", "PresQT_bot/0.1")

	resp, err := TimeoutClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Println("GET", url, "returned", resp.Status)
		return ErrDownload
	}

	targetpath := resolveCache(base)
	f, err := os.Create(targetpath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		os.Remove(targetpath)
		return err
	}
	return nil
}

package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/config"
)

// siteConfigInfo - what app config prints: the site's settings as the
// engine reads them (the config file with the environment over it),
// without its secrets.
type siteConfigInfo struct {
	Env           string `json:"env"`
	Port          int    `json:"port"`
	PublicBaseURL string `json:"public_base_url"`
	Theme         string `json:"theme"`
	EngineDir     string `json:"engine_dir"`
	DB            struct {
		Driver string `json:"driver"`
		Host   string `json:"host"`
		Port   int    `json:"port"`
		Name   string `json:"name"`
	} `json:"db"`
	Files struct {
		PublicDir  string `json:"public_dir"`
		PublicURL  string `json:"public_url"`
		PrivateDir string `json:"private_dir"`
		// Storage, PrivateStorage - "local" or the bucket,
		// s3://bucket/prefix.
		Storage        string `json:"storage"`
		PrivateStorage string `json:"private_storage"`
	} `json:"files"`
}

// runConfig is "app config [-json]": the site's effective settings, no
// secrets - for its administrator, and for app sites reading a site as it
// runs.
func runConfig(args []string) int {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := printConfig(os.Stdout, configInfo(loadConfig()), *asJSON); err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 1
	}
	return 0
}

// configInfo - cfg as siteConfigInfo.
func configInfo(cfg *config.Config) siteConfigInfo {
	var info siteConfigInfo
	info.Env, info.Port, info.PublicBaseURL, info.Theme, info.EngineDir = cfg.Env, cfg.Port, cfg.PublicBaseURL, cfg.Theme, cfg.EngineDir
	info.DB.Driver, info.DB.Host, info.DB.Port, info.DB.Name = cfg.DB.Driver, cfg.DB.Host, cfg.DB.Port, cfg.DB.Scheme
	info.Files.PublicDir, info.Files.PublicURL, info.Files.PrivateDir = cfg.Files.PublicDir, cfg.Files.PublicURL, cfg.Files.PrivateDir
	info.Files.Storage = storageName(cfg.Files.Storage, cfg.Files.S3.Bucket, cfg.Files.S3.Prefix)
	if publicStorage(cfg.Files) == "azure" {
		info.Files.Storage = storageName("azure", cfg.Files.Azure.Container, cfg.Files.Azure.Prefix)
	}
	info.Files.PrivateStorage = storageName(cfg.Files.PrivateStorage, cfg.Files.PrivateS3.Bucket, cfg.Files.PrivateS3.Prefix)
	if strings.EqualFold(strings.TrimSpace(cfg.Files.PrivateStorage), "azure") {
		info.Files.PrivateStorage = storageName("azure", cfg.Files.PrivateAzure.Container, cfg.Files.PrivateAzure.Prefix)
	}
	return info
}

// printConfig writes info to w: JSON, or a line per setting.
func printConfig(w io.Writer, info siteConfigInfo, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}
	_, err := fmt.Fprintf(w, "env: %s\nport: %d\npublic_base_url: %s\ntheme: %s\nengine_dir: %s\ndb: %s %s:%d/%s\nfiles: %s (%s), private %s, storage %s, private storage %s\n",
		info.Env, info.Port, info.PublicBaseURL, info.Theme, info.EngineDir,
		info.DB.Driver, info.DB.Host, info.DB.Port, info.DB.Name,
		info.Files.PublicDir, info.Files.PublicURL, info.Files.PrivateDir, info.Files.Storage, info.Files.PrivateStorage)
	return err
}

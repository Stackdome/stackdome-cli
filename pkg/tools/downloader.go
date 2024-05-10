package tools

import (
	"fmt"
	"runtime"

	getter "github.com/hashicorp/go-getter"
	"github.com/sirupsen/logrus"
)

func DownloadFile(url string, targetPath string) error {
	logrus.Debugf("downloading binary from url: %s", url)
	err := getter.GetAny(targetPath, url)
	if err != nil {
		return err
	}
	return nil
}

func DownloadMutagenBinary(targetDirectory string, version string) error {
	arch := runtime.GOARCH
	os := runtime.GOOS

	url := fmt.Sprintf(
		"https://github.com/mutagen-io/mutagen/releases/download/%s/mutagen_%s_%s_%s.tar.gz",
		version,
		os,
		arch,
		version,
	)
	return DownloadFile(url, targetDirectory)
}

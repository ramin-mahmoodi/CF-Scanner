package task

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
)

const xrayURL = "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-64.zip"

func CheckXrayCore() bool {
	_, err := os.Stat("xray.exe")
	return err == nil
}

func DownloadXrayCore(progressCallback func(string)) error {
	progressCallback("downloading")

	// Create temp file
	tmpFile, err := os.CreateTemp("", "xray-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Download
	resp, err := http.Get(xrayURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return err
	}

	// Close the file so we can unzip it
	tmpFile.Close()

	progressCallback("extracting")

	// Extract
	r, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "xray.exe" || f.Name == "xray" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			outFile, err := os.OpenFile("xray.exe", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, rc)
			if err != nil {
				return err
			}
			break
		}
	}

	return nil
}

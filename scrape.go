package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/firefox"
)

const (
	port = 9559

	downloadURL         = "https://www.abcircle.co.jp/en/downloads/product/CIR115B/"
	downloadButtonXPATH = "//a[ancestor::p[normalize-space()='USB Linux & Mac Driver']]"
)

var (
	sourcePattern = regexp.MustCompile("v([\\d.]+)\\.zip$")
)

func main() {
	downloadDirectory, err := os.MkdirTemp("", "abcccid-download")
	if err != nil {
		log.Fatalln("failed to create temp directory:", err)
	}

	service, err := selenium.NewGeckoDriverService(os.Getenv("GECKODRIVER_BIN"), port)
	if err != nil {
		log.Fatalln("failed to create gecko driver service:", err)
	}

	defer func(service *selenium.Service) {
		_ = service.Stop()
	}(service)

	capabilities := selenium.Capabilities{"browserName": "firefox"}

	firefoxCapabilities := firefox.Capabilities{
		Prefs: map[string]interface{}{
			"browser.download.folderList": 2,
			"browser.download.dir":        downloadDirectory,
		},
		Args: []string{"--headless"},
	}

	firefoxBinary := os.Getenv("FIREFOX_BIN")
	if firefoxBinary != "" {
		firefoxCapabilities.Binary = firefoxBinary
	}

	capabilities.AddFirefox(firefoxCapabilities)

	webDriver, err := selenium.NewRemote(capabilities, fmt.Sprintf("http://localhost:%d", port))
	if err != nil {
		log.Fatalln("failed to create selenium remote:", err)
	}

	defer func(webDriver selenium.WebDriver) {
		_ = webDriver.Close()
	}(webDriver)

	if err := webDriver.Get(downloadURL); err != nil {
		log.Fatalln("failed to get download url:", err)
	}

	downloadButton, err := waitForElement(webDriver, selenium.ByXPATH, downloadButtonXPATH)
	if err != nil {
		log.Fatalln("failed to find download button:", err)
	}

	if _, err := webDriver.ExecuteScript("arguments[0].scrollIntoView(true);", []interface{}{downloadButton}); err != nil {
		log.Fatalln("failed to scroll download button into view:", err)
	}

	if _, err := webDriver.ExecuteScript("arguments[0].click();", []interface{}{downloadButton}); err != nil {
		log.Fatalln("failed to click download button:", err)
	}

	var done bool
	var sourcePath, version string

	for {
		done, sourcePath, version = true, "", ""

		_ = filepath.WalkDir(downloadDirectory, func(filename string, dirEntry fs.DirEntry, err error) error {
			if dirEntry.Type().IsDir() {
				return nil
			}

			if strings.HasSuffix(filename, ".part") {
				done = false
				return nil
			}

			matches := sourcePattern.FindStringSubmatch(filename)
			if len(matches) > 0 {
				sourcePath, version = filename, matches[1]
			}

			return nil
		})

		if done && sourcePath != "" {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	basePath := filepath.Dir(sourcePath)
	targetPath := filepath.Join(basePath, fmt.Sprintf("Circle_Linux_Mac_Driver_v%s.zip", version))

	err = os.Rename(sourcePath, targetPath)
	if err != nil {
		log.Fatalln("failed to rename file:", err)
	}

	outputFilePath := os.Getenv("GITHUB_OUTPUT")
	outputFile, err := os.OpenFile(outputFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalln("failed to open output file:", err)
	}

	defer outputFile.Close()

	if _, err := outputFile.WriteString(fmt.Sprintf("path=%s\nversion=%s\n", targetPath, version)); err != nil {
		log.Fatalln("failed to write output:", err)
	}
}

func waitForElement(webDriver selenium.WebDriver, by, value string) (selenium.WebElement, error) {
	err := webDriver.Wait(func(webDriver selenium.WebDriver) (bool, error) {
		_, err := webDriver.FindElement(by, value)

		return err == nil, nil
	})

	if err != nil {
		return nil, err
	}

	return webDriver.FindElement(by, value)
}

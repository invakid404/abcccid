package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/firefox"
)

const (
	port = 9559

	downloadURL         = "https://www.abcircle.co.jp/en/downloads/product/CIR115B/"
	downloadButtonXPATH = "//a[../../div/div/p='USB Linux & Mac Driver']"
)

var (
	sourcePattern = regexp.MustCompile("Circle_Linux_Mac_Driver_v([\\d.]+)\\.zip$")
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

	sourcePath := ""
	version := ""

	for {
		_ = filepath.WalkDir(downloadDirectory, func(filename string, dirEntry fs.DirEntry, err error) error {
			if dirEntry.Type().IsDir() {
				return nil
			}

			matches := sourcePattern.FindStringSubmatch(filename)
			if len(matches) > 0 {
				sourcePath, version = filename, matches[1]
			}

			return nil
		})

		if sourcePath != "" {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("::set-output name=path::%s\n", sourcePath)
	fmt.Printf("::set-output name=version::%s\n", version)

	// NOTE: webDriver.Quit() hangs when called immediately for some reason.
	//       Give it a bit of time.
	time.Sleep(2 * time.Second)
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

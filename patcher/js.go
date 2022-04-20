package patcher

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"git.sr.ht/~avery/crankshaft/cdp"
	"git.sr.ht/~avery/crankshaft/pathutil"
)

const jsBeautifyBin = "js-beautify"
const libraryRootSP = "libraryroot~sp.js"

/*
PatchJS patches Steam client scripts and reloads the client.

The Steam client overwrites any modified resources at startup, so this must be
run every time Crankshaft starts (if not already patched).

At the moment this only patches libraryroot~sp.js.
*/
func PatchJS(steamuiPath string, debugPort string, serverPort string) error {
	scriptPath := path.Join(steamuiPath, libraryRootSP)

	fmt.Printf("Patching %s...\n", scriptPath)

	checkForOriginal(scriptPath)

	// TODO: use checksum to cache output
	// f, _ := os.Open(scriptPath)
	// defer f.Close()
	// h := md5.New()
	// io.Copy(h, f)
	// fmt.Printf("%x\n", h.Sum(nil))

	// Make a copy of the original, just in case
	copyPath := pathutil.AddExtPrefix(scriptPath, ".orig")
	fmt.Printf("Copying original %s to %s...\n", scriptPath, copyPath)
	err := pathutil.Copy(scriptPath, copyPath)
	if err != nil {
		return err
	}

	fmt.Println("Unminifying...")
	unminFilePath, err := unmin(scriptPath)
	if err != nil {
		return err
	}

	fmt.Printf("Patching class in %s\n", unminFilePath)
	err = patchCoolClass(unminFilePath, scriptPath, serverPort)
	if err != nil {
		return err
	}

	err = reloadClient(debugPort)
	if err != nil {
		return err
	}

	return nil
}

func checkForOriginal(scriptPath string) error {
	f, err := os.Open(scriptPath)
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	// Check first line
	s.Scan()
	firstLine := s.Text()
	if strings.Contains(firstLine, "patched by crankshaft") {
		// Replace with original if possible
		err = pathutil.Copy(pathutil.AddExtPrefix(scriptPath, ".orig"), scriptPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// unmin unminifies the speecifid Javascript file with js-beautify and returns
// the unminified file path.
func unmin(filePath string) (string, error) {
	// foo.js will be unminified to foo.unmin.js
	unminFilePath := pathutil.AddExtPrefix(filePath, ".unmin")

	cmd := exec.Command(jsBeautifyBin, filePath, "-o", unminFilePath)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return unminFilePath, nil
}

/*
patchCoolClass patches a Steam client script to expose internal functionality.

More specifically, it attaches an instance of a specific class to the window so
that Crankshaft scripts can access it. I don't know exactly what the class
does, and the name is minified, but it exposes a lot of cool stuff, so lets
call it coolClass.
*/
func patchCoolClass(unminPath string, origPath string, serverPort string) error {
	// Read the entire file into memory as fileLines for searching and manipulation,
	// then at the end overwrite the original
	fileLines, err := pathutil.FileLines(unminPath)
	if err != nil {
		return err
	}

	constructorLineNum := -1
	constructorLineNum, err = findCoolClassConstructor(fileLines)
	if err != nil {
		fmt.Println("Error finding constructor:", err)
		// return err
	}

	// Add our code

	if constructorLineNum > -1 {
		insertAtPos(&fileLines, constructorLineNum, "window.coolClass = this;")
	}

	script := fmt.Sprintf(`// file patched by crankshaft
		console.info('[Crankshaft] Loading patched libraryroot~sp.js');

		window.addEventListener('load', () => {
			console.info('[Crankshaft] Page loading, making request to inject service');
			fetch('http://localhost:%s/rpc', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify({
					method: 'InjectService.Inject',
					params: [],
					id: Date.now(),
				}),
			})
		});
	`, serverPort)
	insertAtPos(&fileLines, 0, script)

	fmt.Printf("Writing patched file to %s\n", origPath)

	f, err := os.OpenFile(origPath, os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(strings.Join(fileLines, "\n"))
	if err != nil {
		return err
	}

	return nil
}

// findCoolClassConstructor finds which line the constructor for coolClass is
// on.
func findCoolClassConstructor(fileLines []string) (int, error) {
	// First we find a method in the class
	methodExp := regexp.MustCompile(`OpenQuickAccessMenu\(.*\) \{`)
	lineNum := 0
	for _, line := range fileLines {
		lineNum++
		if match := methodExp.MatchString(line); match {
			break
		}
	}

	// Loop over the file backwards, starting from the matched method. We want to
	// find the constructor for the class the method was in
	constructorExp := regexp.MustCompile(`constructor\(.*\) \{`)
	for i := lineNum - 1; i >= 0; i-- {
		if match := constructorExp.MatchString(fileLines[i]); match {
			return i + 1, nil
		}
	}

	return 0, fmt.Errorf("constructor not found")
}

// reloadClient reloads relevant parts of the Steam client to load our
// patched scripts.
func reloadClient(debugPort string) error {
	steamClient, err := cdp.NewSteamClient(debugPort)
	if err != nil {
		return err
	}
	defer steamClient.Cancel()

	err = steamClient.RunScriptInLibrary("window.location.reload()")
	if err != nil {
		return err
	}

	return nil
}

func insertAtPos(arr *[]string, idx int, val string) {
	*arr = append((*arr)[:idx+1], (*arr)[idx:]...)
	(*arr)[idx] = val
}

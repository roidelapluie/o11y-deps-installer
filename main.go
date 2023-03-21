// Copyright 2023 The O11y Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"archive/tar"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/cheggaaa/pb/v3"
)

var (
	//go:embed ansible_alpine.tar.gz
	b []byte
	//go:embed VERSION
	versionContent string
	depsHome       = kingpin.Flag("deps-home", "The destination directory for extracted files").Default("/opt/o11y/deps").String()
	uninstallFlag  = kingpin.Flag("uninstall", "Uninstall the dependencies").Bool()
	reinstallFlag  = kingpin.Flag("reinstall", "Reinstall the dependencies").Bool()
	forceFlag      = kingpin.Flag("force", "Force uninstallation or reinstallation even if there is no O11YDEPSVERSION file").Bool()
)

func main() {
	kingpin.Parse()

	absDepsHome, err := filepath.Abs(*depsHome)
	if err != nil {
		fmt.Println("Error converting the provided path to an absolute path:", err)
		os.Exit(1)
	}

	if absDepsHome == "/" {
		fmt.Println("Error: The provided path is forbidden")
		os.Exit(1)
	}

	depsHome = &absDepsHome

	shouldExit, err := handleUninstallReinstall(*depsHome)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if shouldExit {
		os.Exit(0)
	}

	// Create the destination directory if it doesn't exist
	if err := os.MkdirAll(*depsHome, 0755); err != nil {
		fmt.Println("Error creating destination directory:", err)
		os.Exit(1)
	}

	// Extract the embedded tar.gz file
	if err := extractTarGz(b, *depsHome); err != nil {
		fmt.Println("Error extracting tar.gz file:", err)
		os.Exit(1)
	}

	// Update shebangs
	ansibleVenvPath := filepath.Join(*depsHome, "opt", "ansible-venv")
	if err := updateShebangs(ansibleVenvPath); err != nil {
		fmt.Println("Error updating shebangs:", err)
		os.Exit(1)
	}

	// Update symlinks
	if err := updateSymlinks(ansibleVenvPath); err != nil {
		fmt.Println("Error updating symlinks:", err)
		os.Exit(1)
	}

	patchelfPath := filepath.Join(*depsHome, "usr", "local", "bin", "patchelf")
	if err := fixBinariesAndLibraries(patchelfPath, *depsHome); err != nil {
		fmt.Println("Error fixing binaries and libraries:", err)
		os.Exit(1)
	}

	if err := createWrapperScripts(*depsHome); err != nil {
		fmt.Println("Error creating wrapper scripts:", err)
		os.Exit(1)
	}

	// Write the VERSION file
	if err := writeVersionFile(*depsHome); err != nil {
		fmt.Println("Error writing O11YDEPSVERSION file:", err)
		os.Exit(1)
	}

	fmt.Println("Installation complete.")
}

func extractTarGz(data []byte, dest string) error {
	fmt.Println("Extracting content.")
	// Count the files in the archive
	fileCount := 0
	{
		r := strings.NewReader(string(data))
		gzr, err := gzip.NewReader(r)
		if err != nil {
			return err
		}
		defer gzr.Close()
		tr := tar.NewReader(gzr)
		for {
			_, err := tr.Next()
			if err == io.EOF {
				break // End of archive
			}
			if err != nil {
				return err
			}
			fileCount++
		}
	}

	// Read the tar.gz data
	r := strings.NewReader(string(data))
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	// Create the progress bar
	bar := pb.StartNew(fileCount)

	// Iterate through the files in the archive
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, hdr.Name)

		// Create or extract the file/directory based on the header type
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink, tar.TypeLink:
			err := os.Symlink(hdr.Linkname, target)
			if err != nil {
				return err
			}
		default:
			panic(fmt.Sprintf("Unknown header type: %v", hdr.Typeflag))
		}

		// Increment the progress bar
		bar.Increment()
	}
	bar.Finish()
	return nil
}

func updateShebangs(dir string) error {
	fmt.Println("Updating Shebangs.")
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		shebang := "#!/opt/ansible-venv/bin/python3"
		newShebang := "#!" + filepath.Join(*depsHome, "opt", "ansible-venv", "bin", "python3")
		newContent := strings.Replace(string(content), shebang, newShebang, 1)

		if newContent != string(content) {
			err = ioutil.WriteFile(path, []byte(newContent), info.Mode())
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func updateSymlinks(dir string) error {
	fmt.Println("Updating Symlinks.")
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkDest, err := os.Readlink(path)
			if err != nil {
				return err
			}

			if strings.HasPrefix(linkDest, "/") {
				newLinkDest := filepath.Join(*depsHome, linkDest)

				if err := os.Remove(path); err != nil {
					return err
				}

				if err := os.Symlink(newLinkDest, path); err != nil {
					return err
				}
			}
		}

		return nil
	})

	return err
}

func fixBinariesAndLibraries(patchelfPath, dir string) error {
	fmt.Println("Fixing binaries.")
	// Define the folders to look for binaries and libraries
	folders := []string{
		filepath.Join(dir, "usr", "bin"),
		//		filepath.Join(dir, "lib"),
	}

	dynamicLinker := filepath.Join(dir, "lib", "ld-musl-x86_64.so.1")

	for _, folder := range folders {
		err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("error walking the path %q: %w", path, err)
			}

			if !info.Mode().IsRegular() {
				return nil
			}

			// Skip setting the interpreter for the dynamic linker
			if path == dynamicLinker {
				return nil
			}

			// Check if the file is a binary or a library
			cmd := exec.Command("file", path)
			output, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("error running 'file' command on %q: %v", path, err)
			}

			if strings.Contains(string(output), "ELF") {
				// Fix the binary or library using patchelf
				cmd := exec.Command(patchelfPath, "--set-interpreter", dynamicLinker, path)
				err := cmd.Run()
				if err != nil {
					return fmt.Errorf("error running patchelf on %q: %v", path, err)
				}
			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func createWrapperScripts(depsHome string) error {
	fmt.Println("Creating wrapper scripts.")
	ansibleBinPath := filepath.Join(depsHome, "opt", "ansible-venv", "bin")
	files, err := ioutil.ReadDir(ansibleBinPath)
	if err != nil {
		return fmt.Errorf("Error reading Ansible bin directory: %v", err)
	}

	wrapperDir := filepath.Join(depsHome, "bin")

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "ansible") {
			wrapperPath := filepath.Join(wrapperDir, file.Name())
			wrapperContent := fmt.Sprintf("#!/bin/sh\nLD_LIBRARY_PATH=%s/lib/:%s/usr/lib/ exec %s/opt/ansible-venv/bin/%s \"$@\"\n",
				depsHome, depsHome, depsHome,
				file.Name())

			err := ioutil.WriteFile(wrapperPath, []byte(wrapperContent), 0755)
			if err != nil {
				return fmt.Errorf("Error writing wrapper script for %s: %v", file.Name(), err)
			}
		}
	}
	return nil
}

func handleUninstallReinstall(depsHome string) (bool, error) {
	versionFilePath := filepath.Join(depsHome, "O11YDEPSVERSION")
	data, err := ioutil.ReadFile(versionFilePath)

	versionFileExists := err == nil
	existingVersion := "unknown"
	if versionFileExists {
		existingVersion = strings.TrimSpace(string(data))
	}

	if *uninstallFlag || *reinstallFlag {
		if versionFileExists || *forceFlag {
			err = os.RemoveAll(depsHome)
			if err != nil {
				return false, fmt.Errorf("Error uninstalling: %v", err)
			}

			if *uninstallFlag {
				fmt.Println("Uninstallation complete.")
				return true, nil
			}
		} else {
			files, err := ioutil.ReadDir(depsHome)
			if err != nil {
				return false, fmt.Errorf("Error checking destination directory: %v", err)
			}

			if len(files) > 0 {
				return false, fmt.Errorf("Destination directory exists and is not empty. VERSION: %s. Aborting.", existingVersion)
			}
		}
	} else {
		files, err := ioutil.ReadDir(depsHome)
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("Error checking destination directory: %v", err)
		}

		if len(files) > 0 {
			return false, fmt.Errorf("Destination directory exists and is not empty. VERSION: %s. Aborting.", existingVersion)
		}
	}

	return false, nil
}

// Write the VERSION file to the destination directory
func writeVersionFile(dest string) error {
	fmt.Println("Writing O11YDEPSVERSION file.")
	versionFilePath := filepath.Join(dest, "O11YDEPSVERSION")
	err := ioutil.WriteFile(versionFilePath, []byte(versionContent), 0644)
	if err != nil {
		return fmt.Errorf("Error writing O11YDEPSVERSION file: %v", err)
	}
	return nil
}

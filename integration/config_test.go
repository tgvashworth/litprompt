package integration_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config builds", func() {
	testsDir := findTestsDir()

	entries, err := os.ReadDir(testsDir)
	if err != nil {
		Fail("could not read tests directory: " + err.Error())
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		testName := entry.Name()
		testDir := filepath.Join(testsDir, testName)

		if !fileExists(filepath.Join(testDir, "litprompt.yaml")) &&
			!fileExists(filepath.Join(testDir, "litprompt.yml")) {
			continue
		}

		It("should config-build: "+testName, func() {
			tmpDir := GinkgoT().TempDir()
			copyTree(testDir, tmpDir)

			repoRoot := findRepoRoot()
			binary := filepath.Join(tmpDir, "litprompt")
			buildBin := exec.Command("go", "build", "-o", binary, ".")
			buildBin.Dir = repoRoot
			out, err := buildBin.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "go build failed: "+string(out))

			cmd := exec.Command(binary, "build")
			cmd.Dir = tmpDir
			out, err = cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "litprompt build failed: "+string(out))

			expectedDir := filepath.Join(tmpDir, "expected")
			err = filepath.WalkDir(expectedDir, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil || d.IsDir() {
					return walkErr
				}
				rel, _ := filepath.Rel(expectedDir, path)
				actualPath := filepath.Join(tmpDir, rel)

				expected := mustReadFile(path)
				Expect(actualPath).To(BeAnExistingFile(), "expected output file missing: "+rel)
				actual := mustReadFile(actualPath)
				Expect(actual).To(Equal(expected), "mismatch in "+rel)
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
		})
	}
})

func copyTree(src, dst string) {
	filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		Fail("could not get working directory: " + err.Error())
	}
	for {
		if fileExists(filepath.Join(dir, "go.mod")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			Fail("could not find repo root (go.mod)")
		}
		dir = parent
	}
}

package integration_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tgvashworth/litprompt/internal/build"
)

var _ = Describe("Build", func() {
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

		// Skip directories that don't look like test cases
		srcDir := filepath.Join(testDir, "src")
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			continue
		}

		isErrorCase := fileExists(filepath.Join(testDir, "expected", "error"))

		if isErrorCase {
			It("should fail: "+testName, func() {
				expectedErr := mustReadFile(filepath.Join(testDir, "expected", "error"))
				expectedErr = strings.TrimSpace(expectedErr)

				opts := buildOpts(testDir)
				_, err := build.Build(filepath.Join(srcDir, "prompt.md"), opts)

				Expect(err).To(HaveOccurred(), "expected an error but build succeeded")
				Expect(err.Error()).To(ContainSubstring(expectedErr),
					"error message should contain expected text")
			})
		} else {
			It("should build: "+testName, func() {
				expectedPath := filepath.Join(testDir, "expected", "prompt.md")
				Expect(expectedPath).To(BeAnExistingFile(),
					"expected/prompt.md must exist for non-error test cases")

				expected := mustReadFile(expectedPath)

				opts := buildOpts(testDir)
				actual, err := build.Build(filepath.Join(srcDir, "prompt.md"), opts)

				Expect(err).NotTo(HaveOccurred(), "build should succeed")
				Expect(actual).To(Equal(expected),
					"output should match expected/prompt.md")
			})
		}
	}
})

// findTestsDir walks up from the current directory to find the tests/ directory.
func findTestsDir() string {
	// When running tests, the working directory is the package directory.
	// Walk up to find the repo root containing tests/.
	dir, err := os.Getwd()
	if err != nil {
		Fail("could not get working directory: " + err.Error())
	}

	for {
		candidate := filepath.Join(dir, "tests")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			Fail("could not find tests/ directory")
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func mustReadFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		Fail("could not read file " + path + ": " + err.Error())
	}
	return string(data)
}

func buildOpts(testDir string) build.Options {
	srcDir := filepath.Join(testDir, "src")
	opts := build.Options{
		LockfilePath: filepath.Join(srcDir, "litprompt.lock"),
	}
	mockDir := filepath.Join(testDir, "mock")
	if fileExists(mockDir) {
		opts.MockDir = mockDir
	}
	return opts
}

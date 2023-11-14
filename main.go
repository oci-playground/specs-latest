package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	specsConfigFilename = "specs.yaml"
)

type (
	specsConfig struct {
		Specs []specsConfigSpec `yaml:"specs"`
	}

	specsConfigSpec struct {
		Name     string               `yaml:"name"`
		Remote   string               `yaml:"remote"`
		Releases []specsConfigRelease `yaml:"releases"`
	}

	specsConfigRelease struct {
		Commit string `yaml:"commit"`
		Tag    string `yaml:"tag"`
		Branch string `yaml:"branch"`
	}
)

func main() {
	b, err := os.ReadFile(specsConfigFilename)
	if err != nil {
		log.Fatalf("ReadFile: %v", err)
	}
	config := specsConfig{}
	if err := yaml.Unmarshal(b, &config); err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}
	processConfig(&config)
}

func absPath() string {
	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("os.Getwd: %v", err)
	}
	currentAbsDir, err := filepath.Abs(currentDir)
	if err != nil {
		log.Fatalf("filepath.Abs: %v", err)
	}
	log.Printf("current directory: %s", currentAbsDir)
	return currentAbsDir
}

func processConfig(config *specsConfig) {
	currentDir := absPath()
	gitWorkspace := filepath.Join(currentDir, "docs", "git-workspace")
	log.Printf("ensuring git workspace directory %s", gitWorkspace)
	os.MkdirAll(gitWorkspace, 0755)
	s := `<html>
<head>
<title>OCI specs latest</title>
</head>
<body style="background:#e8e9ff;padding: 20px;font-family: monospace">
<div style="width:100%px; max-width:700px;text-align:left;padding: 20px;border:1px solid #c7c2c2; background:white">
<h1>OCI specs latest</h1>
`
	for _, spec := range config.Specs {
		os.Chdir(gitWorkspace)
		s += processSpec(&spec)
	}
	os.Chdir(currentDir)
	s += `</div>
</body>
</html>
`
	indexFile := filepath.Join(currentDir, "docs", "index.html")
	if err := os.WriteFile(indexFile, []byte(s), 0644); err != nil {
		log.Fatalf("WriteFile: %v", err)
	}
}

func processSpec(spec *specsConfigSpec) string {
	s := fmt.Sprintf("<hr/><h2>%s</h2>\n", spec.Name)
	currentDir := absPath()
	log.Printf("[%s] begin processing, https git remote: %s", spec.Name, spec.Remote)
	base := strings.TrimSuffix(filepath.Base(spec.Remote), ".git")
	specWorkspace := filepath.Join(currentDir, base)
	if _, err := os.Stat(specWorkspace); os.IsNotExist(err) {
		cmd := exec.Command("git", "clone", "--depth=1000000", spec.Remote, specWorkspace)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.Fatalf("cmd.Run: %v", err)
		}
	}
	specOutputParentDir := filepath.Join(currentDir, "..", "specs", spec.Name)
	os.MkdirAll(specOutputParentDir, 0755)
	s += "<ul>"
	for i := len(spec.Releases) - 1; i >= 0; i-- {
		release := spec.Releases[i]
		log.Printf("[%s] count:%d tag:%s branch:%s commit:%s", spec.Name, i+1, release.Commit, release.Tag, release.Branch)
		checkoutTarget := release.Tag
		if checkoutTarget == "" {
			checkoutTarget = release.Commit
			if checkoutTarget == "" {
				log.Fatalf("[%s] invalid config for release at index %d", spec.Name, i)
			}
		}
		s += fmt.Sprintf("<li><div><h3>%s</h3><p>release date: TODO</p></div></li>\n", checkoutTarget)

		specOutputDir := filepath.Join(specOutputParentDir, checkoutTarget)
		if _, err := os.Stat(specOutputDir); !os.IsNotExist(err) {
			log.Printf("[%s] output folder %s exists, skipping", spec.Name, specOutputDir)
			continue
		}

		log.Printf("[%s] changing dir to %s", spec.Name, currentDir)
		os.Chdir(specWorkspace)

		log.Printf("[%s] running \"git checkout -f %s\"", spec.Name, checkoutTarget)
		cmd := exec.Command("git", "checkout", "-f", checkoutTarget)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("cmd.Run: %v", err)
		}
		log.Printf("[%s] running \"git clean -f %s\"", spec.Name, checkoutTarget)
		cmd = exec.Command("git", "clean", "-f", checkoutTarget)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("cmd.Run: %v", err)
		}
		log.Printf("[%s] running \"make docs\"", spec.Name)

		// Ideally "make docs" should be all we need...
		cmd = exec.Command("make", "docs")

		// Various hacks for specific specs... DONT JUDGE ME
		switch spec.Name {
		case "image":
			cmd = exec.Command("sh", "-c", fmt.Sprintf("cat Makefile | sed 's/-it/-i/g' | sed 's|-f gfm|-f gfm --metadata title=\"Open Container Initiative Image Format Specification\"|g' > Makefile.new && (make -f Makefile.new docs || (cd .tool/ && go get github.com/opencontainers/image-spec/specs-go@%s && cd ../ && make -f Makefile.new docs))", checkoutTarget))
		case "distribution":
			cmd = exec.Command("sh", "-c", fmt.Sprintf("cat Makefile | sed 's/-it/-i/g' | sed 's/go mod init \\&\\& \\\\/\\(rm -f go.* \\&\\& go mod init main \\)\\&\\& \\\\/g' | sed 's|-f gfm|-f gfm --metadata title=\"Open Container Initiative Distribution Specification\"|g'> Makefile.new && (make -f Makefile.new docs || (cd .tool/ && go get github.com/opencontainers/distribution-spec/specs-go@%s && cd ../ && make -f Makefile.new docs)) && rm -f output/.gitkeep", checkoutTarget))
		case "runtime":
			cmd = exec.Command("sh", "-c", fmt.Sprintf("cat Makefile | sed 's/-it/-i/g' | sed 's/go mod init \\\\/go mod init main \\\\/g' > Makefile.new && (make -f Makefile.new docs || (cd .tool/ && go get github.com/opencontainers/runtime-spec/specs-go@%s && cd ../ && make -f Makefile.new docs))", checkoutTarget))
		}

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		os.Remove("Makefile.new")
		if err != nil {
			log.Fatalf("cmd.Run: %v", err)
		}

		// move the output contents
		log.Printf("[%s] moving output/ to %s", spec.Name, specOutputDir)
		if err := os.Rename("output/", specOutputDir); err != nil {
			log.Fatalf("os.Rename: %v", err)
		}
	}
	s += "</ul>"
	log.Printf("[%s] changing dir to %s", spec.Name, currentDir)
	os.Chdir(currentDir)
	log.Printf("[%s] end processing", spec.Name)
	return s
}

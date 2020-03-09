package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func InstallDeps(chromiumDir string) {
	if runtime.GOOS == "linux" {
		pth := filepath.Join(chromiumDir, "src", "build", "install-build-deps.sh")
		cmd := exec.Command(pth, "--no-prompt", "--no-nacl", "--no-chromeos-fonts")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}
}

func SetEnvVariable(depotTools string) {
	if runtime.GOOS == "windows" {
		os.Setenv("DEPOT_TOOLS_WIN_TOOLCHAIN", "0")
		os.Setenv("PATH", fmt.Sprintf("%s;%s", os.Getenv("PATH"), depotTools))
	} else {
		os.Setenv("PATH", fmt.Sprintf("%s:%s", os.Getenv("PATH"), depotTools))
	}
}

func GetFFMPEGOS() (osName string) {
	switch runtime.GOOS {
	case "darwin":
		osName = "mac"
		break
	case "linux":
		osName = "linux"
		break
	case "windows":
		osName = "win"
		break
	}
	return
}

func GetFFMPEGArch() (arch string) {
	switch runtime.GOOS {
	case "darwin":
		arch = "x64"
		break
	case "linux":
		arch = "x64"
		break
	case "windows":
		arch = "x64"
		break
	}
	return
}

func main() {

	rootDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(err)
	}

	gclientData := `solutions = [
		{ "name"        : 'src',
			"url"         : 'https://chromium.googlesource.com/chromium/src.git',
			"deps_file"   : 'DEPS',
			"managed"     : False,
			"custom_deps" : {

			},
			"custom_vars": {},
		},
	]`

	buildDir := filepath.Join(rootDir, "dist")
	artifactsDir := filepath.Join(buildDir, "artifacts")
	depotTools := filepath.Join(buildDir, "depot_tools")
	chromiumDir := filepath.Join(buildDir, "chromium")

	ffmpegRoot := filepath.Join(chromiumDir, "src", "third_party", "ffmpeg")

	patchSrc := filepath.Join(rootDir, "ffmpeg.patch")
	patchDst := filepath.Join(ffmpegRoot, "chromium", "scripts", "ffmpeg.patch")

	ffmpegGeneratedGNI := filepath.Join(ffmpegRoot, "ffmpeg_generated.gni")

	chromiumVersion := "80.0.3987.132"

	if _, err := os.Stat(buildDir); !os.IsNotExist(err) {
		fmt.Println("Cleaning....")
		os.RemoveAll(buildDir)
	}

	os.MkdirAll(buildDir, 0755)
	os.MkdirAll(chromiumDir, 0755)
	os.MkdirAll(artifactsDir, 0755)

	// depot_tools
	fmt.Println("Preparing depot_tools....")
	if _, err := os.Stat(depotTools); os.IsNotExist(err) {
		os.MkdirAll(depotTools, 0755)

		cmd := exec.Command("git", "clone", "https://chromium.googlesource.com/chromium/tools/depot_tools.git")
		cmd.Dir = buildDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}

	// update env variable sod epot tools are in the path
	fmt.Println("Set env variables....")
	SetEnvVariable(depotTools)

	// get chromium
	if _, err := os.Stat(chromiumDir); !os.IsNotExist(err) {
		os.MkdirAll(chromiumDir, 0755)
	}

	gclientConfig := filepath.Join(chromiumDir, ".gclient")
	err = ioutil.WriteFile(gclientConfig, []byte(gclientData), os.FileMode(0600))
	if err != nil {
		panic(err)
	}

	cmd := exec.Command("git", "clone", "https://chromium.googlesource.com/chromium/src.git", "--branch", chromiumVersion, "--single-branch", "--depth", "1")
	cmd.Dir = chromiumDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	cmd = exec.Command("git", "reset", "--hard", fmt.Sprintf("tags/%s", chromiumVersion))
	cmd.Dir = filepath.Join(chromiumDir, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	// install build requirements
	InstallDeps(chromiumDir)

	// sync gclient
	cmd = exec.Command("gclient", "sync", "--with_branch_heads")
	cmd.Dir = filepath.Join(chromiumDir, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	// patch ffmpeg
	fmt.Println("Patching codec")
	input, err := ioutil.ReadFile(patchSrc)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(patchDst, input, 0644)
	if err != nil {
		panic(err)
	}

	cmd = exec.Command("git", "apply", "--ignore-space-change", "--ignore-whitespace", "ffmpeg.patch")
	cmd.Dir = filepath.Join(ffmpegRoot, "chromium", "scripts")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	// build ffmpeg
	fmt.Println("Running build_ffmpeg.py")
	cmd = exec.Command("./build_ffmpeg.py", GetFFMPEGOS(), GetFFMPEGArch())
	cmd.Dir = filepath.Join(ffmpegRoot, "chromium", "scripts")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	// generate GN
	fmt.Println("Running generate_gn.py")
	cmd = exec.Command("./generate_gn.py")
	cmd.Dir = filepath.Join(ffmpegRoot, "chromium", "scripts")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	// copy config
	fmt.Println("Running copy_config.sh")
	cmd = exec.Command("./copy_config.sh")
	cmd.Dir = filepath.Join(ffmpegRoot, "chromium", "scripts")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	generatedGNI, err := ioutil.ReadFile(ffmpegGeneratedGNI)
	if err != nil {
		panic(err)
	}

	fmt.Println(fmt.Sprintf("Writing ffmpeg_generated.gni to %s", artifactsDir))
	err = ioutil.WriteFile(filepath.Join(artifactsDir, "ffmpeg_generated.gni"), generatedGNI, 0644)
	if err != nil {
		panic(err)
	}
}

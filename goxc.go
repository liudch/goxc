package main

/*
   Copyright 2013 Am Laher

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

import (
	"archive/zip"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	//Tip for Forkers: please 'clone' from my url and then 'pull' from your url. That way you wont need to change the import path.
	//see https://groups.google.com/forum/?fromgroups=#!starred/golang-nuts/CY7o2aVNGZY
	"github.com/laher/goxc/config"
)


const (
	AMD64   = "amd64"
	X86     = "386"
	ARM     = "arm"
	DARWIN  = "darwin"
	LINUX   = "linux"
	FREEBSD = "freebsd"
	NETBSD  = "netbsd"
	OPENBSD = "openbsd"
	WINDOWS = "windows"
	// Message to install go from source, incase it is missing
	MSG_INSTALL_GO_FROM_SOURCE = "goxc requires Go to be installed from Source. Please follow instructions at http://golang.org/doc/install/source"
)

// Supported platforms
var PLATFORMS = [][]string{
	{DARWIN, X86},
	{DARWIN, AMD64},
	{LINUX, X86},
	{LINUX, AMD64},
	{LINUX, ARM},
	{FREEBSD, X86},
	{FREEBSD, AMD64},
	// {FREEBSD, ARM},
	// couldnt build toolchain for netbsd using a linux 386 host: 2013-02-19
	//	{NETBSD, X86},
	//	{NETBSD, AMD64},
	{OPENBSD, X86},
	{OPENBSD, AMD64},
	{WINDOWS, X86},
	{WINDOWS, AMD64},
}

// settings for this invocation of goxc
var (
// VERSION is initialised by the linker during compilation if the appropriate flag is specified:
// e.g. go build -ldflags "-X main.VERSION 0.1.2-abcd" goxc.go
// thanks to minux for this advice
// So, goxc does this automatically during 'go build'
	VERSION = config.PACKAGE_VERSION_DEFAULT
	settings         config.Settings
	configName       string
	isVersion        bool
	isHelp           bool
	isBuildToolchain bool
	isCliZipArchives string
	isWriteConfig    bool
	isVerbose        bool
)

// this function copied from 'https://github.com/laher/mkdo'
func redirectIO(cmd *exec.Cmd) (*os.File, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Println(err)
	}
	if settings.IsVerbose() {
		log.Printf("Redirecting output")
	}
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)
	//direct. Masked passwords work OK!
	cmd.Stdin = os.Stdin
	return nil, err
}

func sanityCheck(goroot string) error {
	if goroot == "" {
		return errors.New("GOROOT environment variable is NOT set.")
	} else {
		if settings.IsVerbose() {
			log.Printf("Found GOROOT=%s", goroot)
		}
	}
	scriptpath := getMakeScriptPath(goroot)
	_, err := os.Stat(scriptpath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New(fmt.Sprintf("Make script ('%s') does not exist!", scriptpath))
		} else {
			return errors.New(fmt.Sprintf("Error reading make script ('%s'): %v", scriptpath, err))
		}
	}
	return nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func getMakeScriptPath(goroot string) string {
	gohostos := runtime.GOOS
	var scriptname string
	if gohostos == WINDOWS {
		scriptname = "make.bat"
	} else {
		scriptname = "make.bash"
	}
	return filepath.Join(goroot, "src", scriptname)
}

// Build toolchain for a given target platform
func BuildToolchain(goos string, arch string) {
	goroot := runtime.GOROOT()
	scriptpath := getMakeScriptPath(goroot)
	cmd := exec.Command(scriptpath)
	cmd.Dir = filepath.Join(goroot, "src")
	cmd.Args = append(cmd.Args, "--no-clean")
	cgoEnabled := cgoEnabled(goos, arch)

	cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED="+cgoEnabled, "GOARCH="+arch)
	if goos == LINUX && arch == ARM {
		// see http://dave.cheney.net/2012/09/08/an-introduction-to-cross-compilation-with-go
		cmd.Env = append(cmd.Env, "GOARM=5")
	}
	if settings.IsVerbose() {
		log.Printf("'make' env: GOOS=%s CGO_ENABLED=%s GOARCH=%s GOROOT=%s", goos, cgoEnabled, arch, goroot)
		log.Printf("'make' args: %s", cmd.Args)
		log.Printf("'make' working directory: %s", cmd.Dir)
	}
	f, err := redirectIO(cmd)
	if err != nil {
		log.Printf("Error redirecting IO: %s", err)
	}
	if f != nil {
		defer f.Close()
	}
	err = cmd.Start()
	if err != nil {
		log.Printf("Launch error: %s", err)
		return
	}
	err = cmd.Wait()
	if err != nil {
		log.Printf("Wait error: %s", err)
		return
	}
	if settings.IsVerbose() {
		log.Printf("Complete")
	}
}

func moveBinaryToZIP(outDir, binPath, appName string, resources []string, isRemoveBinary bool) (zipFilename string, err error) {
	if settings.PackageVersion != "" && settings.PackageVersion != config.PACKAGE_VERSION_DEFAULT {
		// v0.1.6 using appname_version_platform. See issue 3
		zipFilename = appName + "_" + settings.PackageVersion + "_" + filepath.Base(filepath.Dir(binPath)) + ".zip"
	} else {
		zipFilename = appName + "_" + filepath.Base(filepath.Dir(binPath)) + ".zip"
	}
	zf, err := os.Create(filepath.Join(outDir, zipFilename))
	if err != nil {
		return
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)

	addFileToZIP(zw, binPath)
	if err != nil {
		zw.Close()
		return
	}
	//resources
	for _, resource := range resources {
		addFileToZIP(zw, resource)
		if err != nil {
			zw.Close()
			return
		}
	}
	err = zw.Close()
	if err != nil {
		return
	}
	if isRemoveBinary {
		// Remove binary and its directory.
		err = os.Remove(binPath)
		if err != nil {
			return
		}
		err = os.Remove(filepath.Dir(binPath))
		if err != nil {
			return
		}
	}
	return
}

func addFileToZIP(zw *zip.Writer, path string) (err error) {
	binfo, err := os.Stat(path)
	if err != nil {
		return
	}
	header, err := zip.FileInfoHeader(binfo)
	if err != nil {
		return
	}
	header.Method = zip.Deflate
	w, err := zw.CreateHeader(header)
	if err != nil {
		zw.Close()
		return
	}
	bf, err := os.Open(path)
	if err != nil {
		return
	}
	defer bf.Close()
	_, err = io.Copy(w, bf)
	return
}

func parseIncludeResources(basedir string, includeResources string) []string {
	allMatches := []string{}
	if includeResources != "" {
		resourceGlobs := strings.Split(includeResources, ",")
		for _, resourceGlob := range resourceGlobs {
			matches, err := filepath.Glob(filepath.Join(basedir, resourceGlob))
			if err == nil {
				allMatches = append(allMatches, matches...)
			} else {
				log.Printf("GLOB error: %s: %s", resourceGlob, err)
			}
		}
	}
	if settings.IsVerbose() {
		log.Printf("Resources to include: %v", allMatches)
	}
	return allMatches

}

func signBinary(binPath string) error {
	cmd := exec.Command("codesign")
	cmd.Args = append(cmd.Args, "-s", settings.Codesign, binPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

//0.2.4 refactored this out
func cgoEnabled(goos, arch string) string {
	gohostos := runtime.GOOS
	gohostarch := runtime.GOARCH
	var cgoEnabled string
	if goos == gohostos && arch == gohostarch {
		//note: added conditional in line with Dave Cheney, but this combination is not yet supported.
		if gohostos == FREEBSD && gohostarch == ARM {
			cgoEnabled = "0"
		} else {
			cgoEnabled = "1"
		}
	} else {
		cgoEnabled = "0"
	}
	return cgoEnabled
}

// XCPlat: Cross compile for a particular platform
// 'isFirst' is used simply to determine whether to start a new downloads.md page
// 0.3.0 - breaking change - changed 'call []string' to 'workingDirectory string'.
func XCPlat(goos, arch string, workingDirectory string, isFirst bool) string {
	log.Printf("building for platform %s_%s.", goos, arch)
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		log.Printf("GOPATH env variable not set! Using '.'")
		gopath = "."
	}
	gohostos := runtime.GOOS
	appDirname, err := filepath.Abs(workingDirectory)
	if err != nil {
		log.Printf("Error: %v", err)
	}
	appName := filepath.Base(appDirname)

	relativeDir := filepath.Join(settings.GetFullVersionName(), goos+"_"+arch)

	var outDestRoot string
	if settings.ArtifactsDest != "" {
		outDestRoot = settings.ArtifactsDest
	} else {
		gobin := os.Getenv("GOBIN")
		if gobin == "" {
			// follow usual GO rules for making GOBIN
			gobin = filepath.Join(gopath, "bin")
		}
		outDestRoot = filepath.Join(gobin, appName+"-xc")
	}

	outDir := filepath.Join(outDestRoot, relativeDir)
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("go")
	cmd.Args = append(cmd.Args, "build")
	if settings.GetFullVersionName() != "" {
		cmd.Args = append(cmd.Args, "-ldflags", "-X main.VERSION "+settings.GetFullVersionName()+"")
	}
	cmd.Dir = workingDirectory
	var ending = ""
	if goos == WINDOWS {
		ending = ".exe"
	}
	relativeBinForMarkdown := filepath.Join(goos+"_"+arch, appName+ending)
	relativeBin := filepath.Join(relativeDir, appName+ending)
	cmd.Args = append(cmd.Args, "-o", filepath.Join(outDestRoot, relativeBin), workingDirectory)
	f, err := redirectIO(cmd)
	if err != nil {
		log.Printf("Error redirecting IO: %s", err)
	}
	if f != nil {
		defer f.Close()
	}
	cgoEnabled := cgoEnabled(goos, arch)
	cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED="+cgoEnabled, "GOARCH="+arch)
	if settings.IsVerbose() {
		log.Printf("'go' env: GOOS=%s CGO_ENABLED=%s GOARCH=%s", goos, cgoEnabled, arch)
		log.Printf("'go' args: %v", cmd.Args)
		log.Printf("'go' working directory: %s", cmd.Dir)
	}
	err = cmd.Start()
	if err != nil {
		log.Printf("Launch error: %s", err)
	} else {
		err = cmd.Wait()
		if err != nil {
			log.Printf("Compiler error: %s", err)
		} else {
			log.Printf("Artifact generated OK")

			resources := parseIncludeResources(workingDirectory, settings.Resources.Include)

			// settings.codesign only works on OS X for binaries generated for OS X.
			if settings.Codesign != "" && gohostos == DARWIN && goos == DARWIN {
				if err := signBinary(filepath.Join(outDestRoot, relativeBin)); err != nil {
					log.Printf("codesign failed: %s", err)
				} else {
					log.Printf("Signed with ID: %q", settings.Codesign)
				}
			}

			if settings.IsZipArtifact() {
				// Create ZIP archive.
				zipPath, err := moveBinaryToZIP(
					filepath.Join(outDestRoot, settings.GetFullVersionName()),
					filepath.Join(outDestRoot, relativeBin), appName, resources, !settings.IsBinaryArtifact())
				if err != nil {
					log.Printf("ZIP error: %s", err)
				} else {
					relativeBinForMarkdown = zipPath
					log.Printf("Artifact zipped OK")
				}
			}
			if settings.IsBinaryArtifact() {
				//TODO: move resources to folder & add links to downloads.md
			}

			// Output report
			reportFilename := filepath.Join(outDestRoot, settings.GetFullVersionName(), "downloads.md")
			var flags int
			if isFirst {
				log.Printf("Creating %s", reportFilename)
				flags = os.O_WRONLY | os.O_TRUNC | os.O_CREATE
			} else {
				flags = os.O_APPEND | os.O_WRONLY

			}
			f, err := os.OpenFile(reportFilename, flags, 0600)
			if err == nil {
				defer f.Close()
				if isFirst {
					_, err = fmt.Fprintf(f, "%s downloads (%s)\n------------\n\n", appName, settings.GetFullVersionName())
				}
				_, err = fmt.Fprintf(f, " * [%s %s](%s)\n", goos, arch, relativeBinForMarkdown)
			}
			if err != nil {
				log.Printf("Could not report to '%s': %s", reportFilename, err)
			}
		}
	}
	return relativeBin
}

func printHelp(flagSet *flag.FlagSet) {
	fmt.Fprint(os.Stderr, "`goxc` [options] <directory_name>\n")
	fmt.Fprintf(os.Stderr, " Version '%s'. Options:\n", VERSION)
	flagSet.PrintDefaults()
}

func printVersion(flagSet *flag.FlagSet) {
	fmt.Fprintf(os.Stderr, " goxc version: %s\n", VERSION)
}

//merge configuration file
//maybe oneday: parse source
//TODO honour build flags
func mergeConfiguredSettings(dir string, configName string, useLocal bool) {
	if settings.IsVerbose() {
		log.Printf("loading configured settings")
	}
	configuredSettings, err := config.LoadJsonCascadingConfig(dir, configName, useLocal, settings.IsVerbose())
	if settings.IsVerbose() {
		log.Printf("Settings from config %s: %+v : %v", configName, configuredSettings, err)
	}
	if err == nil {
		settings = config.Merge(settings, configuredSettings)
	}
}

//fillDefaults should happen after writing config
func fillDefaults() {
	settings = config.FillDefaults(settings)
	if settings.IsVerbose() {
		log.Printf("Final settings %+v", settings)
	}
	//v2.0.0: Removed PKG_VERSION parsing
}

// GOXC is the goxc startpoint, having already declared the flags inside 'main'.
// In theory you could call this with a directory name or a list of config files.
func GOXC(call []string) {
	flagSet := setupFlags()
	if err := flagSet.Parse(call[1:]); err != nil {
		log.Printf("Error parsing arguments: %s", err)
		os.Exit(1)
	} else {
		if isVerbose {
			settings.Verbosity = config.VERBOSITY_VERBOSE
		}
		if isBuildToolchain {
			settings.Tasks = []string{config.TASK_BUILD_TOOLCHAIN}
		}
		//0.2.3 NOTE this will be superceded soon
		//using string because that makes it overrideable
		if isCliZipArchives == "true" || isCliZipArchives == "t" {
			settings.ArtifactTypes = []string{config.ARTIFACT_TYPE_ZIP}
		} else if isCliZipArchives == "false" || isCliZipArchives == "f" {
			settings.ArtifactTypes = []string{config.ARTIFACT_TYPE_BIN}
		} else {
			//takes default or takes from config
			settings.ArtifactTypes = []string{}
		}
	}
	//log.Printf("Settings: %s", settings)
	if isHelp {
		printHelp(flagSet)
		os.Exit(0)
	}
	if isVersion {
		printVersion(flagSet)
		os.Exit(0)
	}
	//sanity check
	goroot := runtime.GOROOT()
	if err := sanityCheck(goroot); err != nil {
		log.Printf("Error: %s", err)
		log.Printf(MSG_INSTALL_GO_FROM_SOURCE)
		os.Exit(1)
	}

	args := flagSet.Args()
	var workingDirectory string
	if len(args) < 1 {
		if isBuildToolchain {
			//default to HOME folder
			log.Printf("Building toolchain, so getting config from HOME directory. To use current folder's config, specify the folder (i.e. goxc -t .)");
			workingDirectory = userHomeDir()
		} else {
			log.Printf("Using config from current folder");
			//default to current folder
			workingDirectory = "."
		}
	} else {
		workingDirectory = args[0]
	}
	log.Printf("Config name: %s", configName)

	mergeConfiguredSettings(workingDirectory, configName, !isWriteConfig)

	if isWriteConfig {
		err := config.WriteJsonConfig(workingDirectory, config.WrapJsonSettings(settings), configName, false)
		if err != nil {
			log.Printf("Could not write config file: %v", err)
		}
		// 0.2.5 writeConfig now just exits after writing config
	} else {

		//0.2.3 fillDefaults should only happen after writing config
		fillDefaults()

		if settings.IsVerbose() {
			log.Printf("looping through each platform")
		}
		destOses := strings.Split(settings.Os, ",")
		destArchs := strings.Split(settings.Arch, ",")

		isFirst := true
		for _, supportedPlatformArr := range PLATFORMS {
			supportedOs := supportedPlatformArr[0]
			supportedArch := supportedPlatformArr[1]
			for _, destOs := range destOses {
				if destOs == "" || supportedOs == destOs {
					for _, destArch := range destArchs {
						if destArch == "" || supportedArch == destArch {
							if settings.IsBuildToolchain() {
								BuildToolchain(supportedOs, supportedArch)
							}
							if settings.IsXC() {
								XCPlat(supportedOs, supportedArch, workingDirectory, isFirst)
							}
							isFirst = false
						}
					}
				}
			}
		}
	}
}

// Set up flags.
// Note use of empty strings as defaults, with 'actual' defaults .
// This is done to make merging options from configuration files easier.
func setupFlags() *flag.FlagSet {
	flagSet := flag.NewFlagSet("goxc", flag.ExitOnError)
	flagSet.StringVar(&configName, "c", config.CONFIG_NAME_DEFAULT, "config name (default='.goxc')")
	flagSet.StringVar(&settings.Os, "os", "", "Specify OS (linux,darwin,windows,freebsd,openbsd). Compiles all by default")
	flagSet.StringVar(&settings.Arch, "arch", "", "Specify Arch (386,amd64,arm). Compiles all by default")
	flagSet.StringVar(&settings.PackageVersion, "pv", "", "Package version (usually [major].[minor].[patch]. default='"+config.PACKAGE_VERSION_DEFAULT+"')")
	flagSet.StringVar(&settings.PackageVersion, "av", "", "Package version (deprecated option name)")
	flagSet.StringVar(&settings.PrereleaseInfo, "pi", "", "Prerelease info (usually 'alpha', 'snapshot',...)")
	flagSet.StringVar(&settings.BranchName, "br", "", "Branch name")
	flagSet.StringVar(&settings.BuildName, "bu", "", "Build name")
	flagSet.StringVar(&settings.ArtifactsDest, "d", "", "Destination root directory (default=$GOBIN/(appname)-xc)")
	flagSet.StringVar(&settings.Codesign, "codesign", "", "identity to sign darwin binaries with (only when host OS is OS X)")
	flagSet.StringVar(&settings.Resources.Include, "include", "", "Include resources in zips (default="+config.RESOURCES_INCLUDE_DEFAULT+")") //TODO: Add resources to non-zips & downloads.md

	//0.2.0 Not easy to 'merge' boolean config items. More flexible to translate them to string options anyway
	flagSet.BoolVar(&isBuildToolchain, "t", false, "Build cross-compiler toolchain(s)") //TODO: task types clean,xc,toolchain
	flagSet.BoolVar(&isHelp, "h", false, "Show this help")
	flagSet.BoolVar(&isVersion, "version", false, "version info")
	flagSet.BoolVar(&isVerbose, "v", false, "verbose")
	flagSet.StringVar(&isCliZipArchives, "z", "", "create ZIP archives instead of folders (true/false. default=true)")
	flagSet.BoolVar(&isWriteConfig, "wc", false, "write config (if it doesnt exist. If it does, use another name (with -c option))")
	return flagSet
}

//TODO user-level config file.
func userHomeDir() string {
	usr, err := user.Current()
	if err != nil {
		log.Printf("Could not get home folder: %s", err)
		return os.Getenv("HOME")
	}
	log.Printf("user dir: %s", usr.HomeDir)
	return usr.HomeDir
}

func main() {
	log.SetPrefix("[goxc] ")
	GOXC(os.Args)
}

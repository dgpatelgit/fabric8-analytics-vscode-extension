package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type GoPackages struct {
	Packages []GoPackage `json:"Packages"`
}

type GoModule struct {
	Path    string    `json:"Path"`
	Version string    `json:"Version"`
	Replace *GoModule `json:"Replace"`
}

type GoPackage struct {
	Root       string   `json:"Root"`
	ImportPath string   `json:"ImportPath"`
	Module     GoModule `json:"Module"`
	Standard   bool     `json:"Standard"`
	Imports    []string `json:"Imports"`
	Deps       []string `json:"Deps"`
}

type DirectDeps struct {
	Name        string
	Version     string
	Included    bool
	Packages    []*DirectDeps
	Transitives []*DirectDeps
}

type TransDetails struct {
	Name    string
	Version string
	Module  string
}

type TransDeps struct {
	Name        string
	Version     string
	Module      string
	Transitives []TransDetails
}

// Code root folder
//const rootFolder = "/home/dhpatel/Documents/code/go-learn/sample"
const rootFolder = "/home/dhpatel/Documents/code/go-learn/src/gitops-operator"

var goExecutable string = ""
var goPackages GoPackages
var directDeps = make(map[string]DirectDeps)
var directDepsJson string = ""
var transDepsJson string = ""

func contains(s []string, searchterm string) bool {
	sort.Strings(s)
	i := sort.SearchStrings(s, searchterm)
	return i < len(s) && s[i] == searchterm
}

func getGraphData() int {
	// Get graph data
	cmdGoModGraph := exec.Command(goExecutable, "mod", "graph")
	cmdGoModGraph.Dir = rootFolder
	if output, err := cmdGoModGraph.Output(); err != nil {
		fmt.Printf("ERROR :: Command `%s mod graph` failed, resolve project build errors. %s\n", goExecutable, err)
		return -1
	} else {
		for _, value := range strings.Split(string(output), "\n") {
			if len(value) > 0 {
				pc := strings.Split(value, " ")
				if !strings.Contains(pc[0], "@") {
					mv := strings.Split(pc[1], "@")
					directDeps[mv[0]] = DirectDeps{mv[0], mv[1], false, make([]string, 0)}
				}
			}
		}
		fmt.Println(directDeps)
	}

	return 0
}

func getDepsData() int {
	// Switch to code directory and get graph
	cmdGoListDeps := exec.Command(goExecutable, "list", "-json", "-deps", "./...")
	cmdGoListDeps.Dir = rootFolder
	if output, err := cmdGoListDeps.Output(); err != nil {
		fmt.Println("ERROR :: Command `go list -json -deps ./...` failed, resolve project build errors.", err)
		return -1
	} else {
		goListDepsData := string(output)
		goListDepsData = "{\"Packages\": [" + strings.ReplaceAll(goListDepsData, "}\n{", "},\n{") + "]}"

		json.Unmarshal([]byte(goListDepsData), &goPackages)
		fmt.Println(len(goPackages.Packages))
	}
	return 0
}

func buildDirectDeps(sourceImports []string) {
	directDepsJson = ""
	for mk, mod := range directDeps {
		fmt.Println("Adding packages for direct deps", mod.Name, mod.Version)
		for _, imp := range sourceImports {
			if imp == mod.Name || strings.HasPrefix(imp, mod.Name+"/") {
				fmt.Println(mod.Name, mod.Version, imp)
				om := directDeps[mk]
				if imp == mod.Name {
					om.Included = true
				} else {
					om.Packages = append(directDeps[mk].Packages, imp)
				}
				directDeps[mk] = om
			}
		}
		d, err := json.Marshal(directDeps[mk])
		check(err)
		if len(directDepsJson) == 0 {
			directDepsJson = "\"direct_deps\": [" + string(d)
		} else {
			directDepsJson = directDepsJson + "," + string(d)
		}
	}
	directDepsJson = directDepsJson + "],"
	fmt.Println("Direct Deps :: ", directDeps)
}

func getTransDetails(modPath string, importPath string) []TransDetails {
	var transDetails = make([]TransDetails, 0)
	fmt.Println("Find Trans for importPath :: ", importPath)
	for i := 0; i < len(goPackages.Packages); i++ {
		if goPackages.Packages[i].ImportPath == importPath {
			if goPackages.Packages[i].Standard == true {
				fmt.Println("Skipping strandard import ::", importPath)
				break
			}

			for _, dv := range goPackages.Packages[i].Deps {
				for i := 0; i < len(goPackages.Packages); i++ {
					if goPackages.Packages[i].Standard == false && goPackages.Packages[i].ImportPath == dv /*&& goPackages.Packages[i].Module.Path != modPath*/ {
						transDetails = append(transDetails,
							TransDetails{dv, goPackages.Packages[i].Module.Version, goPackages.Packages[i].Module.Path})
					}
				}
			}
		}
	}
	return transDetails
}

func buildTransitiveDeps() {
	var transDeps = make(map[string]TransDeps)
	transDepsJson = ""

	for _, directDeps := range directDeps {
		fmt.Println("Direct deps ::", directDeps.Name, directDeps.Version, directDeps.Included)
		if directDeps.Included {
			trans := getTransDetails(directDeps.Name, directDeps.Name)
			if len(trans) > 0 {
				transDeps[directDeps.Name] = TransDeps{directDeps.Name, directDeps.Version, directDeps.Name, trans}
				d, err := json.Marshal(transDeps[directDeps.Name])
				check(err)
				if len(transDepsJson) == 0 {
					transDepsJson = "\"transitive_deps\": [" + string(d)
				} else {
					transDepsJson = transDepsJson + "," + string(d)
				}
			}
		}

		for _, pckg := range directDeps.Packages {
			trans := getTransDetails(directDeps.Name, pckg)
			if len(trans) > 0 {
				transDeps[pckg] = TransDeps{pckg, directDeps.Version, directDeps.Name, trans}
				d, err := json.Marshal(transDeps[pckg])
				check(err)
				if len(transDepsJson) == 0 {
					transDepsJson = "\"transitive_deps\": [" + string(d)
				} else {
					transDepsJson = transDepsJson + "," + string(d)
				}
			}
		}
	}
	transDepsJson = transDepsJson + "]"
	fmt.Println("Trans Deps :: ", transDeps)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	// Get 'go' executable path
	goExe, _ := exec.LookPath("go")
	if len(goExe) == 0 {
		fmt.Println("No 'go' executable found on the system")
	} else {
		goExecutable = goExe
		fmt.Println("Found go executable ::", goExecutable)

		if getGraphData() == 0 {
			if getDepsData() == 0 {
				// Get direct imports and deps from current source.
				var sourceImports []string
				for i := 0; i < len(goPackages.Packages); i++ {
					if goPackages.Packages[i].Standard == false && !strings.Contains(goPackages.Packages[i].Root, "@") {
						fmt.Println("ImportPath: " + goPackages.Packages[i].ImportPath + " " + goPackages.Packages[i].Root)
						for _, imp := range goPackages.Packages[i].Imports {
							if !contains(sourceImports, imp) {
								sourceImports = append(sourceImports, imp)
							}
						}
					}
				}
				buildDirectDeps(sourceImports)
				buildTransitiveDeps()

				f, err := os.Create(rootFolder + "/target/clientManifest.json")
				check(err)
				_, err = f.WriteString("{" + directDepsJson + transDepsJson + "}")
				check(err)
				f.Sync()

				defer f.Close()
			}
		}
	}
}

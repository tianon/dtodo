package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	// TODO "github.com/tianon/dtodo/src/dnew"
	"dnew"

	"pault.ag/go/debian/changelog"
	"pault.ag/go/debian/control"
	"pault.ag/go/debian/dependency"
	"pault.ag/go/resolver"
)

func main() {
	log.SetFlags(log.Lshortfile)

	// TODO configurable path?  perhaps allow for an optional *.dsc instead?
	con, err := control.ParseControlFile("debian/control")
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	chg, err := changelog.ParseFileOne("debian/changelog")
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	// TODO configurable or something to avoid guesswork
	targetSuite := chg.Target
	if targetSuite == "UNRELEASED" {
		// check for "Upload to XYZ." in changelog
		re := regexp.MustCompile(`^\s*\*?\s*Upload\s+to\s+(\S+?)\.?(\s+|$)`)
		matches := re.FindStringSubmatch(chg.Changelog)
		if matches != nil {
			targetSuite = matches[1]
		} else {
			targetSuite = "unstable"
		}
	}

	// TODO configurable
	arch := "amd64"

	fmt.Printf("Target: %s\n", targetSuite)
	fmt.Printf("Architecture: %s\n", arch)
	fmt.Printf("Source: %s\n", con.Source.Source)
	fmt.Printf("Version: %s\n", chg.Version)
	fmt.Printf("\n")

	index, err := resolver.GetBinaryIndex(
		"http://httpredir.debian.org/debian",
		targetSuite,
		"main",
		arch,
	)
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}
	// TODO use target suite to include more suites if necessary (ie, "experimental" needs "sid" too)

	incoming, err := resolver.GetBinaryIndex(
		"http://incoming.debian.org/debian-buildd",
		"buildd-"+targetSuite,
		"main",
		arch,
	)
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	newQueue, err := dnew.ParseNewUrl(dnew.New822)
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}
	newBinaries := map[string]dnew.NewEntry{}
	for _, newPkg := range newQueue {
		for _, newBin := range newPkg.Binary {
			newBinaries[newBin] = newPkg
		}
	}

	allDeps := dependency.Dependency{}

	binRelation := dependency.Relation{}
	for _, bin := range con.Binaries {
		binRelation.Possibilities = append(binRelation.Possibilities, dependency.Possibility{
			Name: bin.Package,
			Version: &dependency.VersionRelation{
				Operator: "=",
				Number:   chg.Version.String(),
			},
		})
	}
	allDeps.Relations = append(allDeps.Relations, binRelation)

	allDeps.Relations = append(allDeps.Relations, con.Source.BuildDepends.Relations...)
	allDeps.Relations = append(allDeps.Relations, con.Source.BuildDependsIndep.Relations...)

	for _, bin := range con.Binaries {
		allDeps.Relations = append(allDeps.Relations, bin.Depends.Relations...)
		allDeps.Relations = append(allDeps.Relations, bin.Recommends.Relations...)
		allDeps.Relations = append(allDeps.Relations, bin.Suggests.Relations...)
		allDeps.Relations = append(allDeps.Relations, bin.Enhances.Relations...)
		allDeps.Relations = append(allDeps.Relations, bin.PreDepends.Relations...)
	}

	depArch, err := dependency.ParseArch("any")
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	for _, relation := range allDeps.Relations {
		notes := []string{}
		for _, possi := range relation.Possibilities {
			if possi.Substvar {
				//fmt.Printf("ignoring substvar %s\n", possi)
				continue
			}
			can, why, _ := index.ExplainSatisfies(*depArch, possi)
			if !can {
				inCan, _, _ := incoming.ExplainSatisfies(*depArch, possi)
				if !inCan {
					if newPkg, ok := newBinaries[possi.Name]; ok {
						newUrl := fmt.Sprintf("https://ftp-master.debian.org/new/%s_%s.html", newPkg.Source, newPkg.Version[0])
						notes = append(notes, fmt.Sprintf("NEW (%s): %s", possi.Name, newUrl))
					} else {
						notes = append(notes, why)
					}
				} else {
					notes = append(notes, fmt.Sprintf("%s is in incoming", possi.Name))
				}
			}
		}
		if len(notes) > 0 {
			fmt.Printf("Relation: %s\n", relation)
			if len(notes) > 1 {
				fmt.Printf("Notes:\n %s\n", strings.Join(notes, "\n "))
			} else {
				fmt.Printf("Notes: %s\n", notes[0])
			}
			fmt.Printf("\n")
		}
	}
}

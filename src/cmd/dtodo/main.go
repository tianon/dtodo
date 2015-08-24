package main

import (
	"fmt"
	"log"
	"regexp"

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
	fmt.Printf("Assuming upload against %s.\n", targetSuite)

	arch := "amd64"
	index, err := resolver.GetBinaryIndex(
		"http://httpredir.debian.org/debian",
		targetSuite,
		"main",
		arch,
	)
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

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

	binDeps := dependency.Dependency{}
	for _, bin := range con.Binaries {
		binDeps.Relations = append(binDeps.Relations, &dependency.Relation{
			Possibilities: []*dependency.Possibility{
				{
					Name: bin.Package,
					// TODO add version constraint if information for it is available
				},
			},
		})
	}

	allPossi := binDeps.GetAllPossibilities()
	allPossi = append(allPossi, con.Source.BuildDepends.GetAllPossibilities()...)
	allPossi = append(allPossi, con.Source.BuildDependsIndep.GetAllPossibilities()...)
	for _, bin := range con.Binaries {
		for _, dep := range []dependency.Dependency{
			bin.Depends,
			bin.Recommends,
			bin.Suggests,
			bin.Enhances,
			bin.PreDepends,
		} {
			allPossi = append(allPossi, dep.GetAllPossibilities()...)
		}
	}

	depArch, err := dependency.ParseArch("any")
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	for _, possi := range allPossi {
		can, why, _ := index.ExplainSatisfies(*depArch, possi)
		if !can {
			inCan, _, _ := incoming.ExplainSatisfies(*depArch, possi)
			if !inCan {
				if newPkg, ok := newBinaries[possi.Name]; ok {
					newUrl := fmt.Sprintf("https://ftp-master.debian.org/new/%s_%s.html", newPkg.Source, newPkg.Version[0])
					fmt.Printf("%s: in NEW: %s\n", possi.Name, newUrl)
				} else {
					fmt.Printf("%s: %s\n", possi.Name, why)
				}
			} else {
				fmt.Printf("%s: in incoming!\n", possi.Name)
			}
		}
	}
}

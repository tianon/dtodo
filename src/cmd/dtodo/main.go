package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	// TODO "github.com/tianon/dtodo/src/dnew"
	"dnew"

	"github.com/tianon/go-aptsources"

	"pault.ag/go/debian/changelog"
	"pault.ag/go/debian/control"
	"pault.ag/go/debian/dependency"
	"pault.ag/go/resolver"
)

// TODO configurable; --pedantic ?
const ignoreRelationSecondaryFails = true

type Target struct {
	Mirror     string
	Suites     []string
	Components []string
	Arches     []string

	resolver.Candidates
}

func NewTarget(mirror string, suites, components, arches []string) *Target {
	return &Target{
		Mirror:     mirror,
		Suites:     suites,
		Components: components,
		Arches:     arches,

		Candidates: resolver.Candidates{},
	}
}

func (target Target) AptSource() aptsources.Source {
	return aptsources.Source{
		Types:         []string{"deb"},
		URIs:          []string{target.Mirror},
		Suites:        target.Suites,
		Components:    target.Components,
		Architectures: target.Arches,
	}
}

func (target *Target) Fetch() error {
	can, err := target.AptSource().FetchCandidates()
	if err != nil {
		return err
	}
	target.Candidates = *can
	return nil
}

func (target Target) UrlTo(bin control.BinaryIndex) string {
	return target.Mirror + "/" + bin.Filename
}

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
		// check for "Upload to XYZ." or "Rebuild for XYZ." in changelog
		re := regexp.MustCompile(`^\s*\*?\s*(Upload\s+to|Rebuild\s+for)\s+(\S+?)\.?(\s+|$)`)
		matches := re.FindStringSubmatch(chg.Changelog)
		if matches != nil {
			targetSuite = matches[2]
		} else {
			targetSuite = "unstable"
		}
	}

	// TODO configurable (or auto-sensed from the mirror and/or package source)
	arches := []string{"amd64", "i386"}
	components := []string{"main", "contrib", "non-free"}

	fmt.Printf("Target: %s (%s)\n", targetSuite, chg.Target)
	fmt.Printf("Architectures: %s\n", strings.Join(arches, " "))
	fmt.Printf("Components: %s\n", strings.Join(components, " "))
	fmt.Printf("Source: %s\n", con.Source.Source)
	fmt.Printf("Version: %s\n", chg.Version)
	fmt.Printf("\n")

	indexSources := aptsources.DebianSources(targetSuite, components...)
	index, err := indexSources.FetchCandidates(arches...)
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	incoming := NewTarget(
		"http://incoming.debian.org/debian-buildd",
		[]string{"buildd-" + targetSuite},
		components,
		arches,
	)
	if err = incoming.Fetch(); err != nil {
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

	seenRelations := map[string]bool{}
	for _, relation := range allDeps.Relations {
		relationString := relation.String()
		if seenRelations[relationString] {
			continue
		}
		seenRelations[relationString] = true

		oneCan := false
		notes := []string{}
		for _, possi := range relation.Possibilities {
			if possi.Substvar {
				//fmt.Printf("ignoring substvar %s\n", possi)
				continue
			}
			can, why, _ := index.ExplainSatisfies(*depArch, possi)
			if !can {
				inCan, _, incomingBins := incoming.ExplainSatisfies(*depArch, possi)
				if !inCan {
					if newPkg, ok := newBinaries[possi.Name]; ok {
						newUrl := fmt.Sprintf("https://ftp-master.debian.org/new/%s_%s.html", newPkg.Source, newPkg.Version[0])
						notes = append(notes, fmt.Sprintf("NEW (%s): %s", possi.Name, newUrl))
					} else {
						notes = append(notes, why)
					}
				} else {
					notes = append(notes, fmt.Sprintf("incoming (%s): %s", possi.Name, incoming.UrlTo(incomingBins[0])))
				}
			} else {
				oneCan = true

				// TODO figure out how we can incorporate this ("Section: oldlibs" from debian/control doesn't propagate to the Packages file on the mirror, so we'd have to parse the .deb itself to get this info, which is fairly untenable)
				/*
				// NOTE "bins" is the last return value in the call to "index.ExplainSatisfies" above
				for _, bin := range bins {
					if bin.Section == "oldlibs" {
						oneCan = false
						notes = append(notes, fmt.Sprintf(`%s (%s) is "Section: oldlibs", which suggests it is likely transitional`, bin.Package, bin.Version.String()))
					}
				}
				*/
			}
		}
		if ignoreRelationSecondaryFails && oneCan {
			continue
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

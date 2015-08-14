package main

import (
	"log"

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

	// TODO configurable or something
	suite := "unstable"
	arch := "amd64"
	index, err := resolver.GetBinaryIndex(
		"http://httpredir.debian.org/debian",
		suite,
		"main",
		arch,
	)
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	allCan := true
	allPossi := append(
		con.Source.BuildDepends.GetAllPossibilities(),
		con.Source.BuildDependsIndep.GetAllPossibilities()...,
	)

	depArch, err := dependency.ParseArch("any")
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	allBins := []control.BinaryIndex{}
	for _, possi := range allPossi {
		can, why, bins := index.ExplainSatisfies(*depArch, possi)
		if !can {
			log.Printf("%s: %s\n", possi.Name, why)
			allCan = false
		} else {
			// TODO more smarts for which dep out of bins to use
			allBins = append(allBins, bins[0])
		}
	}

	if !allCan {
		log.Fatalf("Unsatisfied possi; exiting.\n")
	}
}

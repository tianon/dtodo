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

	incoming, err := resolver.GetBinaryIndex(
		"http://incoming.debian.org/debian-buildd",
		"buildd-"+suite,
		"main",
		arch,
	)
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	newQueue, err := ParseNewUrl(New822)
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}
	log.Printf("%d packages in NEW\n", len(newQueue))

	allPossi := append(
		con.Source.BuildDepends.GetAllPossibilities(),
		con.Source.BuildDependsIndep.GetAllPossibilities()...,
	)

	depArch, err := dependency.ParseArch("any")
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	for _, possi := range allPossi {
		can, why, _ := index.ExplainSatisfies(*depArch, possi)
		if !can {
			inCan, _, _ := incoming.ExplainSatisfies(*depArch, possi)
			if !inCan {
				log.Printf("%s: %s\n", possi.Name, why)
			} else {
				log.Printf("%s: in incoming!\n", possi.Name)
			}
		}
	}
}

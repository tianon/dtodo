package main

import (
	"bufio"
	"net/http"

	"pault.ag/go/debian/control"
	"pault.ag/go/debian/version"
)

const New822 = "https://ftp-master.debian.org/new.822"

type NewEntry struct {
	Source        string
	Binary        []string `delim:", "`
	Version       []version.Version
	Architectures []string `delim:", "`
	Age           string
	LastModified  string `control:"Last-Modified"` // TODO make this a time.Time somehow
	Queue         string
	Maintainer    string
	ChangedBy     string `control:"Changed-By"`
	SponsoredBy   string `control:"Sponsored-By"`
	Distribution  string
	Fingerprint   string
	Closes        []string `delim:", "`
	ChangesFile   string   `control:"Changes-File"`
}

func ParseNew(reader *bufio.Reader) ([]NewEntry, error) {
	ret := []NewEntry{}
	err := control.Unmarshal(&ret, reader)
	return ret, err
}

func ParseNewUrl(url string) ([]NewEntry, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ParseNew(bufio.NewReader(resp.Body))
}

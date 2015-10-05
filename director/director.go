package director

import (
	"errors"
)

var (
	NoMatchingServiceError = errors.New("no matching service found")
)

type Director struct {
	domains map[string]*Matcher
}

func NewDirector() *Director {
	return &Director{
		domains: make(map[string]*Matcher),
	}
}

func (d *Director) SetService(domain, prefix, service string) {

	matcher, ok := d.domains[domain]
	if !ok {
		matcher = NewMatcher()
		d.domains[domain] = matcher
	}

	matcher.SetPrefix(prefix, service)
}

func (d *Director) Service(domain, path string) (string, error) {

	matcher, ok := d.domains[domain]
	if !ok {
		return "", NoMatchingServiceError
	}

	return matcher.Match(path)
}

package director

import (
	"errors"
	"strings"
)

var (
	noMatchingPrefixError = errors.New("no matching prefix found")
)

type Matcher struct {
	prefixesList []string
	prefixes     map[string]string
}

func NewMatcher() *Matcher {
	return &Matcher{
		prefixes: make(map[string]string),
	}
}

func (m *Matcher) SetPrefix(prefix, value string) {

	// We only want to add this value to the list if we haven't seen it before.
	if _, ok := m.prefixes[prefix]; !ok {

		// Save a temporary reference to the list and create a new list that has
		// room for another element.
		tmpPrefixesList := m.prefixesList
		m.prefixesList = make([]string, len(m.prefixesList)+1)

		// Find the correct index for the prefix, copying all values up to that point.
		i := 0
		for ; i < len(tmpPrefixesList) && len(tmpPrefixesList[i]) > len(prefix); i++ {
			m.prefixesList[i] = tmpPrefixesList[i]
		}

		// Set the prefix.
		m.prefixesList[i] = prefix

		// Copy the remaining values from the old list.
		for ; i < len(tmpPrefixesList); i++ {
			m.prefixesList[i+1] = tmpPrefixesList[i]
		}
	}

	m.prefixes[prefix] = value
}

func (m *Matcher) Match(path string) (string, string, error) {

	// TODO:
	// match with regex

	// The list of path prefixes is in reverse order by string length. We want
	// to return the first (most specific) match we come accross.
	for _, prefix := range m.prefixesList {
		if strings.HasPrefix(path, prefix) {
			return m.prefixes[prefix], prefix, nil
		}
	}

	return "", "", noMatchingPrefixError
}

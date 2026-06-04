package service

import "strings"

func canonicalizePermissionNames(foundNames, requestedNames []string) ([]string, []string) {
	canonicalNames := make(map[string]string, len(foundNames))
	for _, name := range foundNames {
		canonicalNames[strings.ToLower(name)] = name
	}

	canonicalRequested := make([]string, 0, len(requestedNames))
	missingNames := make([]string, 0)
	for _, name := range requestedNames {
		canonicalName, ok := canonicalNames[strings.ToLower(name)]
		if !ok {
			missingNames = append(missingNames, name)
			continue
		}
		canonicalRequested = append(canonicalRequested, canonicalName)
	}

	return canonicalRequested, missingNames
}

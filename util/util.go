package util

import (
	"errors"
	"math/rand"
	"net/url"
	"strings"
	"time"
)

// Generates the random strings which are used as identifiers for each task
// They need to be large enough to make collisions of tasks not a concern
// Currently the key space is 7.95 * 10^24
func GenRandomIdentifier() string {
	// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
	b := ""
	rand.Seed(time.Now().UTC().UnixNano())
	for i := 0; i < DefaultIdentifierLength; i++ {
		b = b + string(AlphaNumChars[rand.Intn(len(AlphaNumChars))])
	}
	return b
}

// Takes a variety of possible flag formats and puts them
// in a format that chromedp understands (key/value)
func FormatFlag(f string) (string, interface{}, error) {
	if strings.HasPrefix(f, "--") {
		f = f[2:]
	}

	parts := strings.Split(f, "=")
	if len(parts) == 1 {
		return parts[0], true, nil
	} else if len(parts) == 2 {
		return parts[0], parts[1], nil
	} else {
		return "", "", errors.New("Invalid flag: " + f)
	}

}

// Check to see if a flag has been removed by the RemoveBrowserFlags setting
func IsRemoved(toRemove []string, candidate string) bool {
	for _, x := range toRemove {
		if candidate == x {
			return true
		}
	}

	return false
}

// Make a best-effort pass at validating/fixing URLs
func ValidateURL(s string) (string, error) {
	var result string
	u, err := url.ParseRequestURI(s)
	if err != nil {
		if !strings.Contains(s, "://") {
			u, err = url.ParseRequestURI(DefaultProtocolPrefix + s)
			if err != nil {
				return result, errors.New("bad url: " + s)
			}
		} else {
			return result, errors.New("bad url: " + s)
		}
	}

	return u.String(), nil
}

// Sanitize URL for use as a directory or file name
func DirNameFromURL(s string) (string, error) {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return "", err
	}
	return strings.Replace(u.Host+u.EscapedPath(), "/", "-", -1), nil
}

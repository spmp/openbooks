package main

import (
	"crypto/rand"
	"errors"
	"math/big"
	"path"
	"strings"
	"time"

	"github.com/evan-buss/openbooks/server"
)

// Update a server config struct from globalFlags
func bindGlobalServerFlags(config *server.Config) {
	config.UserAgent = globalFlags.UserAgent
	config.UserName = globalFlags.UserName
	config.Log = globalFlags.Log
	config.Server = globalFlags.Server
	config.SearchBot = globalFlags.SearchBot
	config.EnableTLS = globalFlags.EnableTLS
}

// Make sure the server config has a valid rate limit.
func ensureValidRate(rateLimit int, config *server.Config) {

	// If user enters a limit that's too low, set to default of 10 seconds.
	if rateLimit < 10 {
		rateLimit = 10
	}

	config.SearchTimeout = time.Duration(rateLimit) * time.Second
}

func sanitizePath(basepath string) string {
	cleaned := path.Clean(basepath)
	if cleaned == "/" {
		return cleaned
	}
	return cleaned + "/"
}

func applyUsernamePolicy(assignRandomAfter int, userName *string) error {
	*userName = strings.TrimSpace(*userName)

	if assignRandomAfter > 0 {
		if *userName != "" {
			return errors.New("--assign-random-username-after cannot be used with --name")
		}

		*userName = randomAlphaNumeric(12)
		return nil
	}

	if *userName == "" {
		return errors.New("required flag(s) \"name\" not set")
	}

	return nil
}

func randomAlphaNumeric(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)

	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			b[i] = alphabet[i%len(alphabet)]
			continue
		}
		b[i] = alphabet[n.Int64()]
	}

	return string(b)
}

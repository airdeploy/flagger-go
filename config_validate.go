package flagger

import (
	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/log"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"net/url"
	"os"
)

var (
	// ErrBadInitArgs represent bad initialization arguments error
	ErrBadInitArgs = errors.New("bad init arguments")
)

// ENV variable constants
const (
	FlaggerAPIKey          = "FLAGGER_API_KEY"
	FlaggerSourceURL       = "FLAGGER_SOURCE_URL"
	FlaggerBackupSourceURL = "FLAGGER_BACKUP_SOURCE_URL"
	FlaggerSSEUrl          = "FLAGGER_SSE_URL"
	FlaggerIngestionURL    = "FLAGGER_INGESTION_URL"
	FlaggerLogLevel        = "FLAGGER_LOG_LEVEL"
)

func getVarOrEnv(variable, key string) string {
	if variable == "" {
		return os.Getenv(key)
	}
	return variable

}

func validateURL(URL, defaultValue string) (string, error) {
	if URL == "" {
		return defaultValue, nil
	}

	parsedURL, err := url.ParseRequestURI(URL)
	if err != nil {
		return "", ErrBadInitArgs
	}
	return parsedURL.String(), nil
}

func populateURL(urlName, envKey, argsURL, defaultValue, apiKey string) string {
	providedURL := getVarOrEnv(argsURL, envKey)
	if validURL, err := validateURL(providedURL, defaultValue); err != nil {
		log.Errorf("Malformed "+urlName+": %s", providedURL)
		panic(ErrBadInitArgs)
	} else {
		log.Debugf(urlName+": %s", validURL)
		return validURL + apiKey
	}
}

// prepareInitArgs does not mutate provided args *InitArgs returning a copy
func prepareInitArgs(args *InitArgs, info *core.SDKInfo) (_ *InitArgs, err error) {
	if args == nil {
		args = &InitArgs{}
	}
	args = args.copy()
	info = info.Copy()

	args.APIKey = getVarOrEnv(args.APIKey, FlaggerAPIKey)
	if args.APIKey == "" {
		log.Errorf("empty APIKey")
		err = ErrBadInitArgs
	}

	if info.Name == "" {
		log.Errorf("empty SDKInfo.Name")
		err = ErrBadInitArgs
	}

	if info.Version == "" {
		log.Errorf("empty SDKInfo.Version")
		err = ErrBadInitArgs
	}

	defer func() {
		if r := recover(); r != nil {
			err = ErrBadInitArgs
		}
	}()
	args.SourceURL = populateURL("SourceURL", FlaggerSourceURL, args.SourceURL, defaultSourceURL, args.APIKey)
	args.BackupSourceURL = populateURL("BackupSourceURL", FlaggerBackupSourceURL, args.BackupSourceURL, defaultBackupSourceURL, args.APIKey)
	args.SSEURL = populateURL("SSEURL", FlaggerSSEUrl, args.SSEURL, defaultSSEURL, args.APIKey)
	args.IngestionURL = populateURL("IngestionURL", FlaggerIngestionURL, args.IngestionURL, defaultIngestionURL, args.APIKey)

	args.LogLevel = getVarOrEnv(args.LogLevel, FlaggerLogLevel)
	if args.LogLevel == "" {
		args.LogLevel = "error"
	}

	level, parseError := logrus.ParseLevel(args.LogLevel)
	if parseError != nil {
		log.SetLevel(logrus.ErrorLevel)
		log.Errorf("Cannot parse provided logLevel %s, Error level is set", args.LogLevel)
		err = ErrBadInitArgs
	} else {
		log.SetLevel(level)
	}

	return args, err
}

func (args *InitArgs) copy() *InitArgs {
	return &InitArgs{
		APIKey:          args.APIKey,
		SourceURL:       args.SourceURL,
		BackupSourceURL: args.BackupSourceURL,
		SSEURL:          args.SSEURL,
		IngestionURL:    args.IngestionURL,
		LogLevel:        args.LogLevel,
	}
}

package flagger

import (
	"net/url"

	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/log"
	"github.com/pkg/errors"
)

var (
	// ErrBadInitArgs represent bad initialization arguments error
	ErrBadInitArgs = errors.New("bad init arguments")
)

func prepareInitArgs(args *InitArgs, info *core.SDKInfo) (_ *InitArgs, _ *core.SDKInfo, err error) {
	args = args.copy()
	info = info.Copy()

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

	if args.SourceURL == "" {
		args.SourceURL = defaultSourceURL + args.APIKey
	} else if sourceURL, er := url.ParseRequestURI(args.SourceURL); er != nil {
		log.Errorf("bad SourceURL: %s", args.SourceURL)
		err = ErrBadInitArgs
	} else {
		args.SourceURL = sourceURL.String() + args.APIKey
	}
	log.Debugf("SourceURL: %s", args.SourceURL)

	if args.BackupSourceURL == "" {
		args.BackupSourceURL = defaultBackupSourceURL + args.APIKey
	} else if backupSourceURL, er := url.ParseRequestURI(args.BackupSourceURL); er != nil {
		log.Errorf("bad BackupSourceURL: %s", args.BackupSourceURL)
		err = ErrBadInitArgs
	} else {
		args.BackupSourceURL = backupSourceURL.String() + args.APIKey
	}
	log.Debugf("BackupSourceURL: %s", args.BackupSourceURL)

	if args.SSEURL == "" {
		args.SSEURL = defaultSSEURL + args.APIKey
	} else if sseURL, er := url.ParseRequestURI(args.SSEURL); er != nil {
		log.Errorf("bad SSEURL: %s", args.SSEURL)
		err = ErrBadInitArgs
	} else {
		args.SSEURL = sseURL.String() + args.APIKey
	}

	log.Debugf("SSEURL: %s", args.SSEURL)

	if args.IngestionURL == "" {
		args.IngestionURL = defaultIngestionURL + args.APIKey
	} else if ingestionURL, er := url.ParseRequestURI(args.IngestionURL); er != nil {
		log.Errorf("bad IngestionURL: %s", args.IngestionURL)
		err = ErrBadInitArgs
	} else {
		args.IngestionURL = ingestionURL.String() + args.APIKey
	}

	log.Debugf("IngestionURL: %s", args.IngestionURL)

	return args, info, err
}

func (args *InitArgs) copy() *InitArgs {
	return &InitArgs{
		APIKey:          args.APIKey,
		SourceURL:       args.SourceURL,
		BackupSourceURL: args.BackupSourceURL,
		SSEURL:          args.SSEURL,
		IngestionURL:    args.IngestionURL,
	}
}

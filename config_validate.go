package flagger

import (
	"net/url"

	"github.com/airdeploy/flagger-go/core"
	"github.com/airdeploy/flagger-go/log"
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
		log.Warnf("empty SourceURL, default is used: %s", defaultSourceURL)
		args.SourceURL = defaultSourceURL
	} else if _, er := url.ParseRequestURI(args.SourceURL); er != nil {
		log.Errorf("bad SourceURL: %s", args.SourceURL)
		err = ErrBadInitArgs
	}

	if args.BackupSourceURL == "" {
		log.Warnf("empty BackupSourceURL, default is used: %s", defaultBackupSourceURL)
		args.BackupSourceURL = defaultBackupSourceURL
	} else if _, er := url.ParseRequestURI(args.BackupSourceURL); er != nil {
		log.Errorf("bad BackupSourceURL: %s", args.BackupSourceURL)
		err = ErrBadInitArgs
	}

	if args.SSEURL == "" {
		log.Warnf("empty SSEURL, default is used: %s", defaultSSEURL)
		args.SSEURL = defaultSSEURL
	} else if _, er := url.ParseRequestURI(args.SSEURL); er != nil {
		log.Errorf("bad SSEURL: %s", args.SSEURL)
		err = ErrBadInitArgs
	}

	if args.IngestionURL == "" {
		log.Warnf("empty IngestionURL, default is used: %s", defaultIngestionURL)
		args.IngestionURL = defaultIngestionURL
	} else if _, er := url.ParseRequestURI(args.IngestionURL); er != nil {
		log.Errorf("bad IngestionURL: %s", args.IngestionURL)
		err = ErrBadInitArgs
	}

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

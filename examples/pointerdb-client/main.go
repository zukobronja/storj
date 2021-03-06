// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"os"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "storj.io/storj/pkg/paths"
	client "storj.io/storj/pkg/pointerdb"
	proto "storj.io/storj/protos/pointerdb"
)

var (
	pointerdbClientPort string
)

func initializeFlags() {
	flag.StringVar(&pointerdbClientPort, "pointerdbPort", ":8080", "this is your port")
	flag.Parse()
}

func main() {
	initializeFlags()

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	pdbclient, err := client.NewClient(pointerdbClientPort)

	if err != nil {
		logger.Error("Failed to dial: ", zap.Error(err))
		os.Exit(1)
	}

	logger.Debug(fmt.Sprintf("client dialed port %s", pointerdbClientPort))
	ctx := context.Background()

	// Example parameters to pass into API calls
	var path = p.New("fold1/fold2/fold3/file.txt")
	pointer := &proto.Pointer{
		Type:          proto.Pointer_INLINE,
		InlineSegment: []byte("popcorn"),
	}
	APIKey := []byte("abc123")

	// Example Put1
	err = pdbclient.Put(ctx, path, pointer, APIKey)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("couldn't put pointer in db", zap.Error(err))
	} else {
		logger.Debug("Success: put pointer in db")
	}

	// Example Put2
	err = pdbclient.Put(ctx, p.New("fold1/fold2"), pointer, APIKey)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("couldn't put pointer in db", zap.Error(err))
	} else {
		logger.Debug("Success: put pointer in db")
	}

	// Example Get
	getRes, err := pdbclient.Get(ctx, path, APIKey)

	if err != nil {
		logger.Error("couldn't GET pointer from db", zap.Error(err))
	} else {
		logger.Info("Success: got Pointer from db",
			zap.String("pointer", getRes.String()),
		)
	}

	// Example List with pagination
	startingPathKey := p.New("fold1/")
	var limit int64 = 1

	paths, trunc, err := pdbclient.List(ctx, startingPathKey, limit, APIKey)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to list file paths", zap.Error(err))
	} else {
		var stringList []string
		for _, pathByte := range paths {
			stringList = append(stringList, string(pathByte))
		}
		logger.Debug("Success: listed paths: " + strings.Join(stringList, ", ") + "; truncated: " + fmt.Sprintf("%t", trunc))
	}

	// Example Delete
	err = pdbclient.Delete(ctx, path, APIKey)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("Error in deleteing file from db", zap.Error(err))
	} else {
		logger.Debug("Success: file is deleted from db")
	}
}

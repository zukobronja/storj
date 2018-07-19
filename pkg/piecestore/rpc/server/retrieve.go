// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"errors"
	"io"
	"log"
	"os"

	"storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/utils"
	pb "storj.io/storj/protos/piecestore"
)

// Retrieve -- Retrieve data from piecestore and send to client
func (s *Server) Retrieve(stream pb.PieceStoreRoutes_RetrieveServer) error {
	log.Println("Retrieving data...")

	// Receive Signature
	recv, err := stream.Recv()
	if err != nil || recv == nil {
		log.Println(err)
		return errors.New("Error receiving Piece data")
	}

	pd := recv.GetPieceData()
	log.Printf("ID: %s, Size: %v, Offset: %v\n", pd.GetId(), pd.GetSize(), pd.GetOffset())

	// Get path to data being retrieved
	path, err := pstore.PathByID(pd.GetId(), s.DataDir)
	if err != nil {
		return err
	}

	// Verify that the path exists
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Read the size specified
	totalToRead := pd.Size
	// Read the entire file if specified -1
	if pd.Size <= -1 {
		totalToRead = fileInfo.Size()
	}

	retrieved, allocated, err := s.retrieveData(stream, pd.GetId(), pd.GetOffset(), totalToRead)
	if err != nil {
		return err
	}

	log.Printf("Successfully retrieved data: Allocated: %v, Retrieved: %v\n", allocated, retrieved)
	return nil
}

func (s *Server) retrieveData(stream pb.PieceStoreRoutes_RetrieveServer, id string, offset, length int64) (retrieved, allocated int64, err error) {
	storeFile, err := pstore.RetrieveReader(stream.Context(), id, offset, length, s.DataDir)
	if err != nil {
		return 0, 0, err
	}

	defer utils.Close(storeFile)

	writer := NewStreamWriter(s, stream)
	var totalRetrieved, totalAllocated int64
	var allocations []int64
	for totalRetrieved < length {
		// Receive Bandwidth allocation
		recv, err := stream.Recv()
		if err != nil {
			log.Println(err)
			return 0, 0, err
		}

		ba := recv.GetBandwidthallocation()
		baData := ba.GetData()

		if baData != nil {
			if err = s.verifySignature(ba.GetSignature()); err != nil {
				return 0, 0, err
			}

			if err = s.writeBandwidthAllocToDB(ba); err != nil {
				return 0, 0, err
			}

			allocation := baData.GetSize()
			if allocation < 0 {
				allocation = 1024 * 32 // 32 kb
			}

			allocations = append(allocations, allocation)
			totalAllocated += allocation
		}

		if len(allocations) <= 0 {
			continue
		}

		sizeToRead := allocations[len(allocations)-1]

		if sizeToRead > length-totalRetrieved {
			sizeToRead = length - totalRetrieved
		}

		buf := make([]byte, sizeToRead) // buffer size defined by what is being allocated
		n, err := storeFile.Read(buf)
		if err == io.EOF {
			break
		}
		// Write the buffer to the stream we opened earlier
		n, err = writer.Write(buf[:n])
		if err != nil {
			return 0, 0, err
		}
		totalRetrieved += int64(n)
		allocations[len(allocations)-1] -= int64(n)

		if allocations[len(allocations)-1] <= 0 {
			allocations = allocations[:len(allocations)-1]
		}
	}

	return totalRetrieved, totalAllocated, nil
}

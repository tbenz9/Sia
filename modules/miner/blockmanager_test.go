package miner

import (
	"bytes"
	"testing"

	"github.com/NebulousLabs/Sia/crypto"
	"github.com/NebulousLabs/Sia/modules"
	"github.com/NebulousLabs/Sia/types"
)

// solveHeader takes a block header as input and returns a solved block header
// as output.
func solveHeader(header types.BlockHeader, target types.Target) types.BlockHeader {
	// Solve the header.
	for {
		// Incrememnt the nonce first to guarantee that a new header is formed
		// - this helps check for pointer errors.
		header.Nonce[0]++
		id := crypto.HashObject(header)
		if bytes.Compare(target[:], id[:]) >= 0 {
			break
		}
	}
	return header
}

// TestIntegrationHeaderForWork checks that header requesting, solving, and
// submitting naively works.
func TestIntegrationHeaderForWork(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	mt, err := createMinerTester("TestIntegreationHeaderForWork")
	if err != nil {
		t.Fatal(err)
	}

	// Get a header and solve it.
	header, target, err := mt.miner.HeaderForWork()
	if err != nil {
		t.Fatal(err)
	}
	solvedHeader := solveHeader(header, target)
	// Sanity check - header and solvedHeader should be different. (within the
	// testing file, 'header' should always have a nonce of '0' and
	// solvedHeader should never have a nonce of '0'.
	if header.Nonce == solvedHeader.Nonce {
		t.Fatal("nonce memory is not independent")
	}

	// Submit the header.
	err = mt.miner.SubmitHeader(solvedHeader)
	if err != nil {
		t.Fatal(err)
	}
}

// TestIntegrationHeaderForWorkUpdates checks that HeaderForWork starts
// returning headers on the new block after a block has been submitted to the
// consensus set.
func TestIntegrationHeaderForWorkUpdates(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	mt, err := createMinerTester("TestIntegreationHeaderForWorkUpdates")
	if err != nil {
		t.Fatal(err)
	}

	// Get a header to advance into the header memory.
	_, _, err = mt.miner.HeaderForWork()
	if err != nil {
		t.Fatal(err)
	}

	// Submit a block, which should trigger a header change.
	_, err = mt.miner.AddBlock()
	if err != nil {
		t.Fatal(err)
	}

	// Get a header to grind on.
	header, target, err := mt.miner.HeaderForWork()
	if err != nil {
		t.Fatal(err)
	}
	solvedHeader := solveHeader(header, target)

	// Submit the header.
	err = mt.miner.SubmitHeader(solvedHeader)
	if err != nil {
		t.Fatal(err)
	}
	if !mt.cs.InCurrentPath(types.BlockID(crypto.HashObject(solvedHeader))) {
		t.Error("header from solved block is not in the current path")
	}
}

// TestIntegrationManyHeaders checks that requesting a thousand headers in
// a row results in a thousand unique headers, and all of them can be
// reassebled into valid blocks.
func TestIntegrationManyHeaders(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	mt, err := createMinerTester("TestIntegrationManyHeaders")
	if err != nil {
		t.Fatal(err)
	}

	// Create a suite of headers for imaginary parallel mining.
	solvedHeaders := make([]types.BlockHeader, HeaderMemory/BlockMemory*2)
	for i := range solvedHeaders {
		header, target, err := mt.miner.HeaderForWork()
		if err != nil {
			t.Fatal(err)
		}
		solvedHeaders[i] = solveHeader(header, target)
	}

	// Submit the headers randomly and make sure they are all considered valid.
	selectionOrder := crypto.Perm(len(solvedHeaders))
	for _, selection := range selectionOrder {
		err = mt.miner.SubmitHeader(solvedHeaders[selection])
		if err != nil && err != modules.ErrNonExtendingBlock {
			t.Error(err)
		}
	}
}

// TestIntegrationHeaderBlockOverflow triggers a header overflow by requesting
// a block that triggers the overflow.
func TestIntegrationHeaderBlockOverflow(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	mt, err := createMinerTester("TestIntegrationHeaderBlockOverflow")
	if err != nil {
		t.Fatal(err)
	}

	// Grab a header that will be overwritten.
	header, target, err := mt.miner.HeaderForWork()
	if err != nil {
		t.Fatal(err)
	}
	header = solveHeader(header, target)

	// Mine blocks to wrap the memProgress around and wipe the old header.
	for i := 0; i < BlockMemory; i++ {
		_, err = mt.miner.AddBlock()
		if err != nil {
			t.Fatal(err)
		}
		// Grab a header to advance the mempool progress.
		_, _, err = mt.miner.HeaderForWork()
		if err != nil {
			t.Fatal(err)
		}
	}

	// Previous header should no longer be in memory.
	err = mt.miner.SubmitHeader(header)
	if err != errLateHeader {
		t.Error(err)
	}
}

// TestIntegrationHeaderRequestOverflow triggers a header overflow by
// requesting a header that triggers overflow.
func TestIntegrationHeaderRequestOverflow(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	mt, err := createMinerTester("TestIntegrationHeaderRequestOverflow")
	if err != nil {
		t.Fatal(err)
	}

	// Grab a header that will be overwritten.
	header, target, err := mt.miner.HeaderForWork()
	if err != nil {
		t.Fatal(err)
	}
	header = solveHeader(header, target)

	// Mine blocks to bring memProgress up to the edge. The number is chosen
	// specifically so that the overflow happens during the requesting of 200
	// headers.
	for i := 0; i < BlockMemory-1; i++ {
		_, err = mt.miner.AddBlock()
		if err != nil {
			t.Fatal(err)
		}
		// Grab a header to advance the mempool progress.
		_, _, err = mt.miner.HeaderForWork()
		if err != nil {
			t.Fatal(err)
		}
	}

	// Header should still be in memory.
	err = mt.miner.SubmitHeader(header)
	if err != modules.ErrNonExtendingBlock {
		t.Error(err)
	}

	// Request headers until the overflow is achieved.
	for i := 0; i < HeaderMemory/BlockMemory; i++ {
		_, _, err = mt.miner.HeaderForWork()
		if err != nil {
			t.Fatal(err)
		}
	}

	err = mt.miner.SubmitHeader(header)
	if err != errLateHeader {
		t.Error(err)
	}
}

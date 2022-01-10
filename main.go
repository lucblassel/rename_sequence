package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/biogo/biogo/alphabet"
	"github.com/biogo/biogo/io/seqio/fasta"
	"github.com/biogo/biogo/seq/linear"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/hillbig/rsdic"
)

type Record struct {
	Name     string `json:"name"`
	Offsets  []byte `json:"offsets"`
	sequence string
}

func getNewPos(pos uint64, offsets *rsdic.RSDic) uint64 {
	rank := offsets.Rank(pos, true)
	bit := offsets.Bit(pos)
	if bit {
		return rank
	}
	return rank - 1
}

func getNewName(name string, refs map[string]*rsdic.RSDic) (string, error) {
	nameArray := strings.Split(name, "!")
	newNameArray := nameArray

	startPos, err := strconv.ParseUint(nameArray[2], 10, 64)
	if err != nil {
		return "", err
	}
	endPos, err := strconv.ParseUint(nameArray[3], 10, 64)
	if err != nil {
		return "", err
	}

	ref := refs[nameArray[1]]

	newNameArray[2] = fmt.Sprintf("%d", getNewPos(startPos, ref)+1)
	newNameArray[3] = fmt.Sprintf("%d", getNewPos(endPos, ref)+1)

	return strings.Join(newNameArray, "!"), nil
}

func main() {
	sequencesPath := flag.String("sequences", "", "path to sequences to reduce in .fasta format")
	offsetsPath := flag.String("offsetsPath", "", "path to save correspondence between original positions and reduced positions of reference")
	outputPath := flag.String("output", "", "path to output reduced sequences in .fasta format")

	flag.Parse()

	if *offsetsPath == "" || *sequencesPath == "" || *outputPath == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Read Fasta file
	reads, err := parseFasta(*sequencesPath)
	if err != nil {
		log.Fatalf("Could not parse input Fasta: %v", err)
	}

	// Organize reads by name
	readMap := make(map[string]*Record, len(reads))
	for _, read := range reads {
		readMap[read.Name] = read
	}

	// Parse offsets
	offsetsFile, err := os.Open(*offsetsPath)
	if err != nil {
		log.Fatalf("Could not parse input offsets: %v", err)
	}
	defer offsetsFile.Close()
	decoder := json.NewDecoder(offsetsFile)

	var offsets []*Record
	decoder.Decode(&offsets)

	// Decode offsets to succinct data structure
	refs := make(map[string]*rsdic.RSDic, len(offsets))
	for _, offset := range offsets {
		ref := rsdic.New()
		if err := ref.UnmarshalBinary(offset.Offsets); err != nil {
			log.Fatalf("Error decoding offsets: %v", err)
		}
		refs[offset.Name] = ref
	}

	// Create fasta writer
	outputFasta, err := os.Create(*outputPath)
	if err != nil {
		log.Fatalf("Couldn't create output FASTA: %v", err)
	}
	defer outputFasta.Close()
	fastaWriter := fasta.NewWriter(outputFasta, 80)

	// Decode offsets and write them to new fasta
	for _, record := range reads {
		newName, err := getNewName(record.Name, refs)
		if err != nil {
			log.Fatalf("Error renaming read: %v", err)
		}
		seq := linear.NewSeq(newName, []alphabet.Letter(readMap[record.Name].sequence), alphabet.DNA)
		_, err = fastaWriter.Write(seq)
		if err != nil {
			log.Fatalf("Error writing sequence: %v", err)
		}
	}

}

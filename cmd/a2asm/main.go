package main

import (
	"fmt"
	"log"
	"os"

	"github.com/taeber/a2asm"
)

var usage = `Apple //e Assembler

Usage: a2asm <ASSEMBLY_FILE>

Code is NOT given a 4-byte, DOS 3.3 style header. (ORG, LEN: uint16)

Converts MERLIN-type assembly into 6502 binary.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stdout, usage)
		os.Exit(2)
	}

	fp := os.Stdin

	if os.Args[1] != "-" {
		var err error
		if fp, err = os.Open(os.Args[1]); err != nil {
			log.Fatalln(err)
		}
	}

	if _, err := a2asm.Assemble(os.Stdout, fp); err != nil {
		log.Fatalln(err)
	}
}

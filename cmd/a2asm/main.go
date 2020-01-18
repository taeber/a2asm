package main

import (
	"log"
	"os"

	"github.com/taeber/a2asm"
)

var usage = `
Apple //e Assembler

Usage: a2asm <PROGRAM >PROGRAM.B

Code is given a DOS 3.3 style header.

Converts MERLIN-type assembly into 6502 binary.
`

func main() {
	_, err := a2asm.Assemble(os.Stdout, os.Stdin)
	return
	fp, err := os.Open("/home/taeber/code/a2asm/6502progs/bell.s")
	if err != nil {
		log.Fatalln(err)
	}
	_, err2 := a2asm.Assemble(os.Stdout, fp)
	if err2 != nil {
		log.Fatalln(err2)
	}
}

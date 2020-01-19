package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/taeber/a2asm"
)

var usage = `Apple //e Assembler

Usage: a2asm [-headless] <ASSEMBLY_FILE>

Converts MERLIN-type assembly into 6502 binary. A 4-byte, DOS 3.3 header
comprising the origin and length is prefixed unless -headless is used.

`

var headless = flag.Bool("headless", false, "do not write the DOS 3.3 header")

func main() {
	flag.Usage = func() {
		fmt.Print(usage)
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	fp := os.Stdin

	src := flag.Arg(0)
	if src != "-" {
		var err error
		if fp, err = os.Open(src); err != nil {
			log.Fatalln(err)
		}
	}

	n, err := a2asm.Assemble(os.Stdout, fp, *headless)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println(n, "bytes written")
}

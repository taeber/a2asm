a2asm - A simple 6502 assembler
===============================

`a2asm` is a 6502 assembler written in Go and licensed under the MIT License.

As mentioned in [Juiced.GS](https://juiced.gs/index/v26/i3/?target=issue-links),
it supports a "subset of the classic Merlin assembler syntax... sufficient to
assemble all of the examples from Roger Wagner's _Assembly Lines_ book."

Copyright 2020, Taeber Rapczak <taeber@rapczak.com>.


Quickstart
----------

    $ go build -o a2asm cmd/a2asm/main.go
    $ ./a2asm --help

    Usage: a2asm [-headless] <ASSEMBLY_FILE>

    Converts MERLIN-type assembly into 6502 binary. A 4-byte, DOS 3.3 header
    comprising the origin and length is prefixed unless -headless is used.

    $ ./a2asm 6502progs/bell.s >BELL.A2


Tips
----

After editing your program, you can use [AppleCommander][] and [LinApple][] to
test it out on an emulated Apple //e:

    $ edit 6502progs/hello.s
    $ java -jar ac.jar -e disk.dsk HELLO
    10  PRINT  CHR$(4);"BRUN TEST"
    $ java -jar ac.jar -d disk.dsk TEST
    $ ./a2asm 6502progs/hello.s | java -jar ac.jar -dos disk.dsk TEST B
    $ linapple --autoboot --conf $PWD/linapple.conf --d1 $PWD/disk.dsk


[AppleCommander]: https://applecommander.github.io/
[LinApple]: https://github.com/linappleii/linapple/


Testing
-------

To run the unit tests, you can simply execute:

    $ go test

To compare against the _Assembly Lines_ programs, you'll need to extract them
from the DSK images to the current folder then run:

    $ ./compareall.bash


Motivation
----------

I wanted to learn 6502 assembly on a recently acquired Apple //e. I bought
_[Assembly Lines](https://ct6502.org/product/assembly-lines-the-complete-book/)_
by Roger Wagner (edited by Chris Torrence) and was introduced to the Merlin
assembler. It was great to learn with.

When I decided to try and write my own programs, I found it easier to iterate
using VIM and LinApple on my Ubuntu laptop. I found [CC65][] and their CA65
assembler with its modern ideas on programming the 6502. I was soon lost in
their documentation and eventually ended up switching to using C as it seemed
to be the easier path. After porting about 50% of the line-editor `ed` to the
Apple //e, however, I found out that the C code that had worked in my emulator,
didn't work on the real hardware!

Somewhere along the way I forgot that I was just trying to learn 6502 assembly
and maybe using a "modern" assembler wasn't the appropriate tool for the job.

So, I wrote this assembler. I wrote tests to ensure every program of Roger
Wagner assembles correctly. I have not added any more features, so while it
uses Merlin-style syntax, it is not 100% Merlin compatible.

[CC65]: https://cc65.github.io/

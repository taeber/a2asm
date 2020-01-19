a2asm - A simple 6502 assembler
===============================

`a2asm` is written in Go and is licensed under the MIT License.

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

After editing your program, you can use [AppleCommander]() and [LinApple]() to
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

I bought _Assembly Lines_ by Roger Wagner and enjoyed using the MERLIN
assembler on my Apple //e to start learning 6502 assembly.

When I decided to try and write my own program, I found it easier to use VIM on
my Ubuntu laptop. I found [CC65]() and their CA65 assembler with its modern ideas
on programming the 6502. I was soon lost in their documentation (which is
pretty good). I eventually ended up switching to using C as it seemed like the
easier path. However, after porting about 50% of the line-editor ed to the
Apple //e, I found out that the code didn't work on the real hardware!

Somewhere along the way I forgot that I was just trying to learn 6502 assembly
and maybe using a "modern" assembler wasn't the appropriate tool for the job.

So, I wrote this assembler. I wrote tests to ensure every program of Roger
Wagner assembles correctly. I have not added any more features, so while it
uses MERLIN-style syntax, it is not 100% MERLIN compatible.

[CC65](https://cc65.github.io/)


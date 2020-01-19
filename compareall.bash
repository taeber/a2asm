#!/bin/bash
#
# Test the assembly of all programs from "Assembly Lines" by Roger Wagner
#

# First, see if we can assemble all the "Assembly Lines" programs.
for each in $(ls AssemblyLinesWagnerDOS{1,2}/*.S)
do
    ./a2asm $each >/dev/null;
done 2>&1 | grep -B 1 'Line ' && exit

# Now, compare the reported output with a2asm's output using hexdump and cmp.
for each in $(ls AssemblyLinesWagnerDOS{1,2}/*.txt)
do
    echo $each
    if [ ! -f "${each%.*}.S" ]
    then
        echo Skipping... ${each%.*}.S does not exist.
        continue
    fi
    ORIG=$(cat $each | grep '^\$' | awk '{ for (i=2; i<18; i++) if ($i != "..") printf $i }')
    MINE=$(./a2asm -headless ${each%.*}.S 2>/dev/null | hexdump -v -e '/1 "%02X"')

    cmp <(echo -n $ORIG) <(echo -n $MINE) || {
        cat ${each%.*}.S
        cat $each
        ./a2asm -headless ${each%.*}.S 2>/dev/null | hexdump -C
        echo "./a2asm -headless ${each%.*}.S 2>/dev/null | hexdump -C"
        exit
    }
done

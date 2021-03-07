.PHONY: clean a2asm test

a2asm:
	go build -o a2asm cmd/a2asm/main.go

ac.jar:
	curl -L 'https://github.com/AppleCommander/AppleCommander/releases/download/v1-5-0/AppleCommander-ac-1.5.0.jar' >ac.jar

disk.dsk:
	curl -L 'https://github.com/AppleWin/AppleWin/raw/master/bin/MASTER.DSK' >disk.dsk

test:
	go test
	./compareall.bash

clean:
	rm -f a2asm ac.jar disk.dsk

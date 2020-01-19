package a2asm

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// Modes:
//  1. Immediate #$
//  2. Absolute $HHHH
//  3. Zero Page $HH
//  4. Implicit/Implied
//  5. Relative
//  6. Indexed  ,X
//  7. Indirect Indexed ($HH),Y
//  8. Indexed Indirect ($HH,X)

func TestPrg1(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
      ORG $300
BELL  EQU $FBDD
*
START JSR BELL
      RTS
	`)
	expected := []byte("\x20\xDD\xFB\x60")
	Assemble(out, prg, true)
	actual := out.Bytes()
	if !bytes.Equal(expected, actual) {
		t.Errorf("Expected %v; got %v", expected, actual)
	}
}

func TestPrg2(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
      ORG $300
COUT  EQU $FDED
HI    EQU $80
*
START LDA #$D4  "T"
      JSR COUT
	  JMP DONE
DONE  RTS
	  DFB HI,8,%10000000
	  HEX 112233
	`)
	expected := []byte("\xA9\xD4\x20\xED\xFD\x4C\x08\x03\x60\x80\x08\x80\x11\x22\x33")
	_, err := Assemble(out, prg, true)
	if err != nil {
		t.Error(err)
		return
	}

	actual := out.Bytes()
	if !bytes.Equal(expected, actual) {
		t.Errorf("Expected %v; got %v", expected, actual)
	}
}

func TestBranch(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
START LDX #1
	  BEQ DONE
	  DEX
	  BEQ START
DONE  RTS
	`)
	_, err := Assemble(out, prg, true)
	if err != nil {
		t.Error(err)
		return
	}

	expected := []byte("\xA2\x01\xF0\x03\xCA\xF0\xF9\x60")
	actual := out.Bytes()
	if !bytes.Equal(expected, actual) {
		t.Errorf("Expected %v; got %v", expected, actual)
	}
}

func TestDFB(t *testing.T) {
	test(t, " DFB $12", "\x12")
	test(t, " DFB $12,$34", "\x12\x34")
}

func TestJSR(t *testing.T) {
	test(t, " JSR $1234", "\x20\x34\x12")
}

func TestLDA(t *testing.T) {
	test(t, " LDA #$12", "\xA9\x12")
	test(t, " LDA #12", "\xA9\x0C")
}

func TestRTS(t *testing.T) {
	test(t, " RTS", "\x60")
}

func TestParseOperand(t *testing.T) {
	var mode addressingMode
	var val []byte
	var err error

	check := func(expMode addressingMode, expVal string) {
		if err != nil {
			t.Error(err)
			return
		}

		if mode != expMode {
			t.Errorf("Wrong mode. Expected %v; got %v", expMode, mode)
			return
		}

		if string(val) != expVal {
			t.Errorf("wrong value. Expected %s; got %s", expVal, val)
			return
		}
	}

	mode, val, err = parseOperand([]byte("042"))
	check(absolute, "042")

	mode, val, err = parseOperand([]byte("BELL  Jumps to BELL"))
	check(absolute, "BELL")

	mode, val, err = parseOperand([]byte("BELL+1  COMMENT"))
	check(absolute, "BELL+1")

	mode, val, err = parseOperand([]byte("$1234,X"))
	check(absoluteX, "$1234")

	mode, val, err = parseOperand([]byte("#1234  This is a comment"))
	check(immediate, "1234")

	mode, val, err = parseOperand([]byte("($4321)"))
	check(indirect, "$4321")

	mode, val, err = parseOperand([]byte("($40,X)"))
	check(indexedIndirect, "$40")

	mode, val, err = parseOperand([]byte("($40),Y"))
	check(indirectIndex, "$40")

	mode, val, err = parseOperand([]byte("#$12"))
	check(immediate, "$12")
}

func TestParseOperandValue(t *testing.T) {
	var num uint16
	var ref string

	check := func(expNum uint16, expRef string) {
		if num != expNum {
			t.Errorf("expected: %d; got %d", expNum, num)
			return
		}

		if string(ref) != expRef {
			t.Errorf("expected: %s; got %s", expRef, ref)
			return
		}
	}

	num, ref, _ = parseOperandValue([]byte("BELL+1"))
	check(1, "BELL")

	num, ref, _ = parseOperandValue([]byte("$0000+15"))
	check(0x000F, "")

	num, ref, _ = parseOperandValue([]byte("BELL-1"))
	check(0xFFFF, "BELL")

	num, ref, _ = parseOperandValue([]byte("$0A  ; BUFFER PTR"))
	check(0x0A, "")
}

func test(t *testing.T, assembly, expected string) {
	s := state{
		Reader: bufio.NewReader(strings.NewReader(assembly)),
		Labels: make(map[string]address),
	}

	if err := parseLine(&s); err != nil {
		t.Error(err)
		return
	}

	actual := s.Memory[0:len(expected)]
	if !bytes.Equal(s.Memory[0:len(expected)], []byte(expected)) {
		for _, b := range actual {
			t.Logf("%x ", b)
		}
		t.Errorf(" len=%d\n", len(actual))
		return
	}
}

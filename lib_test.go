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
//  9. Accumulator (ex: ROL A) (ex: ROL)

func TestPrg1(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
      ORG $300
BELL  EQU $FBDD
*
	  LDA #>START
START JSR BELL
      RTS
	  LDA #>START
	`)
	expected := []byte("\xA9\x03\x20\xDD\xFB\x60\xA9\x03")
	Assemble(out, prg, true)
	actual := out.Bytes()
	if !bytes.Equal(expected, actual) {
		t.Errorf("Expected %v; got %v", expected, actual)
	}
}

func TestPrg1AltEQU(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
      ORG $300
BELL  = $FBDD
*
	  LDA #>START
START JSR BELL
      RTS
	  LDA #>START
	`)
	expected := []byte("\xA9\x03\x20\xDD\xFB\x60\xA9\x03")
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
	  HEX 112233    ; THIS IS A COMMENT
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

func TestZeroPageRef(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
      ORG $300
CSW   EQU $36
VECT  EQU $3EA
*
START LDA #ENTRY
	  STA CSW
	  LDA #>ENTRY
	  STA CSW+1
	  JMP VECT
*
ENTRY RTS
	`)
	expected := []byte("\xA9\x0B\x85\x36\xA9\x03\x85\x37\x4c\xEA\x03\x60")
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

func TestAccumulatorMode(t *testing.T) {
	test(t, " ROL $44", "\x26\x44")
	test(t, " ROL A", "\x2A")
	test(t, " ASL", "\x0A")
	test(t, " ROL ;With a comment", "\x2A")
	test(t, " LSR", "\x4A")
	test(t, " ROR", "\x6A")
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

func TestLocalLabels(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
START LDX #1
	  BEQ :DONE
	  DEX
	  BEQ START
:DONE RTS
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

func TestLabelEQULabel(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
LOMEM EQU $0912
FMT   EQU LOMEM
PTR   EQU $06
SRC   EQU PTR
      LDX #0
      LDA #0
	  STA FMT,X
	  STA SRC,X
	`)
	_, err := Assemble(out, prg, true)
	if err != nil {
		t.Error(err)
		return
	}

	expected := []byte("" +
		"\xA2\x00" +
		"\xA9\x00" +
		"\x9D\x12\x09" +
		"\x95\x06")

	actual := out.Bytes()
	if !bytes.Equal(expected, actual) {
		t.Errorf("Expected %v; got %v", expected, actual)
	}
}

func TestASCII(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
START LDX #'T'
	  BEQ EXIT
	  DEX
	  BEQ START
EXIT  RTS
      LDX #"T"
	`)
	_, err := Assemble(out, prg, true)
	if err != nil {
		t.Error(err)
		return
	}

	expected := []byte("\xA2\x54\xF0\x03\xCA\xF0\xF9\x60\xA2\xD4")
	actual := out.Bytes()
	if !bytes.Equal(expected, actual) {
		t.Errorf("Expected %v; got %v", expected, actual)
	}
}

func TestJumpEqu(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
	ORG $800
	JSR MAIN
	JMP EXIT
EXIT	EQU $3D0
INIT	EQU $FB2F
HOME	EQU $FC58
MAIN	JSR INIT
	JSR HOME
	RTS
	`)
	_, err := Assemble(out, prg, true)
	if err != nil {
		t.Error(err)
		return
	}

	expected := []byte("\x20\x06\x08\x4C\xD0\x03\x20\x2F\xFB\x20\x58\xFC\x60")
	actual := out.Bytes()
	if !bytes.Equal(expected, actual) {
		t.Errorf("Expected %v; got %v", expected, actual)
	}
}

func TestArithmetic(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
	ORG $800
MAIN	LDA #MAIN+$A+2*2
	`)

	_, err := Assemble(out, prg, true)
	if err != nil {
		t.Error(err)
		return
	}

	expected := []byte("\xA9\x18")
	actual := out.Bytes()
	if !bytes.Equal(expected, actual) {
		t.Errorf("Expected %v; got %v", expected, actual)
	}
}

func TestLabelAliases(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
				ORG $8000
EXIT            EQU $3D0
COUT            EQU $FDED
CROUT           EQU $FD8E
				JSR CROUT
* IFNE #$01 #$01
*   A2_1 A2_2
A2_0            LDA #$01
				CMP #$01
				BNE A2_1
				JMP A2_2
A2_1            RTS
* IFEQ #$01 #$01
*   A2_4 A2_5
A2_3            LDA #$01
				CMP #$01
				BEQ A2_4
				JMP A2_5
* COPYBB @A #"O"
A2_4            LDA #"O"
				JSR COUT
* COPYBB @A #"K"
				LDA #"K"
				JSR COUT
A2_5            JSR EXIT
A2_2            EQU A2_3
	`)

	_, err := Assemble(out, prg, true)
	if err != nil {
		t.Error(err)
		return
	}

	/*
		20 8e fd
		a9 01
		c9 01
		d0 03
		4c 0d 80
		60
		a9 01
		c9 01
		f0 03
		4c 20 80
		a9 cf
		20 ed fd
		a9 cb
		20 ed fd
		20 d0 03
	*/
	expected := []byte(
		"\x20\x8E\xFD\xA9\x01\xC9\x01\xD0\x03\x4C\x0D\x80\x60\xA9\x01\xC9\x01\xF0\x03\x4C\x20\x80\xA9\xCF\x20\xED\xFD\xA9\xCB\x20\xED\xFD\x20\xD0\x03")
	//	                                           00  00  A2_2 is being set to $0000 instead of A2_3 ($080D)!

	actual := out.Bytes()
	if !bytes.Equal(expected, actual) {
		t.Errorf("Expected %v; got %v", expected, actual)
	}
}

func TestCurrentAddress(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
		ORG $8000
MAIN	LDX #0
		BEQ *+2+1
		INX
		JMP *
	`)

	_, err := Assemble(out, prg, true)
	if err != nil {
		t.Error(err)
		return
	}

	// 8000-	A2 00   	LDX #$00
	// 8002-	F0 01   	BEQ $8005
	// 8004-	E8      	INX
	// 8005-	4C 05 80	JMP $8005
	expected := []byte("\xA2\x00\xF0\x01\xE8\x4C\x05\x80")

	actual := out.Bytes()
	if !bytes.Equal(expected, actual) {
		t.Errorf("Expected %v; got %v", expected, actual)
	}
}

func TestHiLoArithmetic(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
		ORG $8000
NAME	EQU $24B1
		LDA #<NAME+$1E
		LDX #>NAME+$1E
	`)

	_, err := Assemble(out, prg, true)
	if err != nil {
		t.Error(err)
		return
	}

	// 8000-	A9 CF   	LDA #<NAME+$1E
	// 8002-	A2 24   	LDX #>NAME+$1E
	expected := []byte("\xA9\xCF\xA2\x24")

	actual := out.Bytes()
	if !bytes.Equal(expected, actual) {
		t.Errorf("Expected %v; got %v", expected, actual)
	}
}

func TestHiAsciiDash(t *testing.T) {
	out := bytes.NewBuffer(nil)
	prg := strings.NewReader(`
		ORG $8000
		LDA #"-"
	`)

	_, err := Assemble(out, prg, true)
	if err != nil {
		t.Error(err)
		return
	}

	// 8000-	A9 AD   	LDA #"-"
	expected := []byte("\xA9\xAD")

	actual := out.Bytes()
	if !bytes.Equal(expected, actual) {
		t.Errorf("Expected %v; got %v", expected, actual)
	}
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

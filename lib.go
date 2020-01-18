package a2asm

// Super simple assembler
import (
	"bufio"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"io"
)

func Assemble(dst io.Writer, src io.Reader) (written uint, err error) {
	s := state{
		Reader:     bufio.NewReader(src),
		Labels:     make(map[string]address),
		Constants:  make(map[string]uint16),
		References: make(map[string][]*reference),
	}

	for err == nil {
		err = parseLine(&s)
	}

	if err != io.EOF {
		return
	}

	err = nil

	for lbl := range s.References {
		addr, ok := s.Labels[lbl]
		if !ok {
			err = fmt.Errorf("unknown label: %s", lbl)
			return
		}

		for _, ref := range s.References[lbl] {
			pos := ref.Address
			num := binary.LittleEndian.Uint16(s.Memory[pos:])
			if !ref.Relative {
				binary.LittleEndian.PutUint16(s.Memory[pos:], num+addr)
				continue
			}

			s.Memory[pos] = uint8(num + (addr + 1) - (pos - 1))
		}
	}

	dst.Write(s.Memory[s.Address-s.Written : s.Address])
	written = uint(s.Written)

	return
}

// address is a location in the 6502's main memory.
type address = uint16

type state struct {
	Reader     *bufio.Reader
	Labels     map[string]address
	Constants  map[string]uint16
	References map[string][]*reference

	Memory  [0xFFFF]byte
	Address address
	Written uint16

	LineNumber uint
	Line       []byte

	Label string
}

type reference struct {
	Address  address
	Relative bool
}

type addressingMode uint

const (
	absolute  addressingMode = iota // or Zero Page or Relative
	absoluteX                       // or Zero Page X
	absoluteY                       // or Zero Page Y
	immediate
	indexedIndirect // ($12,X)
	indirectIndex   // ($12),Y
	indirect        // JMP ($1234)
)

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func isHex(ch byte) bool {
	return isDigit(ch) || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F')
}

func isLetter(ch byte) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z')
}

func readLabel(line []byte) (label string, remaining []byte) {
	remaining = line
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			continue
		}

		label = string(line[0:i])
		remaining = line[i:]

		break
	}

	return
}

func readMneumonic(line []byte) (mneumonic string, remaining []byte) {
	remaining = line
	i := 0

	for ; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			break
		}
	}

	if i+3 > len(line) {
		return
	}

	mneumonic = string(line[i : i+3])
	mneumonic = strings.ToUpper(mneumonic)
	i = i + 3

	for ; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			break
		}
	}

	remaining = line[i:]
	return
}

func readNumber(text []byte) (uint16, []byte, error) {
	if text[0] == '$' {
		// Read hex literal.
		num, err := strconv.ParseUint(string(text[1:]), 16, 16)
		if err != nil {
			return 0, text, err
		}

		var i int
		for i = 1; i < len(text) && isHex(text[i]); i++ {
		}

		return uint16(num), text[i:], err
	}

	if isDigit(text[0]) {
		num, err := strconv.ParseUint(string(text), 10, 16)
		if err != nil {
			return 0, text, err
		}

		var i int

		for i = 0; i < len(text) && isDigit(text[i]); i++ {
		}

		return uint16(num), text[i:], err
	}

	return 0, text, fmt.Errorf("expected hex or decimal literal; got %s", text)
}

func parseLine(s *state) (err error) {
	var isPrefix bool

	if s.Line, isPrefix, err = s.Reader.ReadLine(); err != nil {
		return
	}

	s.LineNumber++

	if isPrefix {
		err = fmt.Errorf("line %d is too long", s.LineNumber)
		return
	}

	if len(s.Line) == 0 || s.Line[0] == '*' || strings.Trim(string(s.Line), "\t ") == "" {
		// Skip empty and comment lines.
		return
	}

	line := s.Line

	var label string
	label, line = readLabel(line)

	var mneumonic string
	mneumonic, line = readMneumonic(line)

	switch mneumonic {
	case "ORG":
		s.Address, line, err = readNumber(line)
		return

	case "EQU":
		var def uint16
		def, line, err = readNumber(line)
		s.Constants[label] = def
		return

	case "CHK":
		// TODO: implement
		return

	case "HEX":
		// TODO: implement
		return

	case "ASC":
		// TODO: implement
		// TODO: lo-ascii 'HI'
		// TODO: hi-ascii "HI"
		return
	}

	if label != "" {
		s.Labels[label] = s.Address
	}

	switch mneumonic {
	case "DEX":
		s.write(0xCA)
	case "DEY":
		s.write(0x88)
	case "INX":
		s.write(0xE8)
	case "INY":
		s.write(0xC8)

	case "TAX":
		s.write(0xAA)
	case "TXA":
		s.write(0x8A)
	case "TAY":
		s.write(0xA8)
	case "TYA":
		s.write(0x98)
	case "TSX":
		s.write(0xBA)
	case "TXS":
		s.write(0x9A)

	case "PLA":
		s.write(0x68)
	case "PHA":
		s.write(0x48)
	case "PLP":
		s.write(0x28)
	case "PHP":
		s.write(0x08)

	case "BRK":
		s.write(0x00)
	case "RTI":
		s.write(0x40)
	case "RTS":
		s.write(0x60)

	case "CLC":
		s.write(0x18)
	case "SEC":
		s.write(0x38)
	case "CLD":
		s.write(0xD8)
	case "SED":
		s.write(0xF8)
	case "CLI":
		s.write(0x58)
	case "SEI":
		s.write(0x78)
	case "CLV":
		s.write(0xB8)
	case "NOP":
		s.write(0xEA)
	default:
		goto TRYMORE
	}

	return

TRYMORE:
	var mode addressingMode
	var value []byte
	mode, value, err = parseOperand(line)
	if err != nil {
		return
	}

	var num uint16
	var ref string
	num, ref, err = parseOperandValue(value)

	var refAdded *reference

	if ref != "" {
		if def, ok := s.Constants[ref]; ok {
			num = def
		} else if refAddr, ok := s.Labels[ref]; ok {
			num += refAddr
		} else {
			refAdded = &reference{s.Address + 1, false}
			s.References[ref] = append(s.References[ref], refAdded)
		}
	}

	switch mneumonic {
	case "LDA":
		switch mode {
		case immediate:
			s.write(0xA9)
			s.writeShort(num)
		case indexedIndirect:
			s.write(0xA1)
			s.writeShort(num)
		case indirectIndex:
			s.write(0xB1)
			s.writeShort(num)
		case absoluteX:
			if num < 0xFF {
				// Zero Page,X
				s.write(0xA5)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0xBD)
			s.writeNumber(num)
		case absoluteY:
			// Absolute,Y
			s.write(0xB9)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF {
				// Zero Page
				s.write(0xA5)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0xAD)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "STA": //todo
	case "LDX":
		switch mode {
		case immediate:
			s.write(0xA2)
			s.writeShort(num)
		case absoluteY:
			if num < 0xFF {
				s.write(0xB6)
				s.writeShort(num)
				break
			}
			s.write(0xBE)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF {
				// Zero Page
				s.write(0xA6)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0xAE)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "STX": //todo
	case "LDY": //todo
	case "STY": //todo
	case "JMP":
		switch mode {
		case absolute:
			s.write(0x4C)
			s.writeNumber(num)
		case indirect:
			s.write(0x6C)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}
	case "BIT": //todo

	//todo: all Logical and arithmetic commands

	case "BPL":
		s.write(0x10)
	case "BMI":
		s.write(0x30)
	case "BVC":
		s.write(0x50)
	case "BVS":
		s.write(0x70)
	case "BCC":
		s.write(0x90)
	case "BCS":
		s.write(0xB0)
	case "BNE":
		s.write(0xD0)
	case "BEQ":
		s.write(0xF0)
		s.writeShort(num - s.Address)
		if refAdded != nil {
			refAdded.Relative = true
		}

	case "JSR":
		if mode != absolute {
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}
		s.write(0x20)
		s.writeNumber(num)

	default:
		err = fmt.Errorf(`unknown mneumonic: "%s"`, mneumonic)
		return
	}

	return
}

func parseOperand(text []byte) (mode addressingMode, val []byte, err error) {
	var i int

	if text[0] == '#' {
		mode = immediate
		for i = 1; i < len(text); i++ {
			if text[i] == ' ' {
				break
			}
		}
		val = text[1:i]
		return
	}

	if text[0] != '(' {
		for i = 0; i < len(text); i++ {
			ch := text[i]

			if ch == ' ' {
				break
			}

			if ch == ',' {
				switch text[i+1] {
				case 'X':
					mode = absoluteX
				case 'Y':
					mode = absoluteY
				default:
					err = fmt.Errorf("invalid character after comma")
				}
				break
			}
		}
		val = text[0:i]

		return
	}

	// text[0] == '('
	for i = 1; i < len(text); i++ {
		ch := text[i]
		if ch == ')' {
			val = text[1:i]
			mode = indirect

			if i+1 == len(text) || text[i+1] == ' ' {
				return
			}

			if i+2 >= len(text) || text[i+1] != ',' || text[i+2] != 'Y' {
				err = fmt.Errorf("expected ),Y")
				return
			}

			mode = indirectIndex
			return
		}

		if ch == ',' {
			if i+2 >= len(text) || text[i+1] != 'X' || text[i+2] != ')' {
				err = fmt.Errorf("expected ,X)")
				return
			}

			val = text[1:i]
			mode = indexedIndirect

			return
		}
	}

	err = fmt.Errorf("missing rparen")
	return
}

func parseOperandValue(val []byte) (num uint16, ref string, err error) {
	end := len(val)
	for i, ch := range val {
		if ch == '-' || ch == '+' {
			end = i
			break
		}
	}

	if isLetter(val[0]) {
		ref = string(val[0:end])
	} else {
		num, _, err = readNumber(val[0:end])
		if err != nil {
			return
		}
	}

	val = val[end:]

	if len(val) == 0 {
		return
	}

	var num2 uint16
	switch val[0] {
	case ' ':
	case '+':
		num2, _, err = readNumber(val[1:])
		if err != nil {
			return
		}
		num += num2
	case '-':
		num2, _, err = readNumber(val[1:])
		if err != nil {
			return
		}
		num -= num2
	default:
		err = fmt.Errorf("invalid +/- offset")
		return
	}

	return
}

func (s *state) write(b byte) {
	s.Memory[s.Address] = b
	s.Address++
	s.Written++
}

func (s *state) writeShort(num uint16) {
	s.write(byte(num & 0xFF))
}

func (s *state) writeNumber(num uint16) {
	binary.LittleEndian.PutUint16(s.Memory[s.Address:], num)
	s.Address += 2
	s.Written += 2
}

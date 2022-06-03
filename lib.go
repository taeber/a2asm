package a2asm

// Super simple assembler
import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"io"
)

const highASCII = 0b1000_0000

// Assemble reads MERLIN-style 6502 assembly from src and writes the
// corresponding binary to dst. It returns how many bytes were written or if
// an error (err) occurred.
func Assemble(dst io.Writer, src io.Reader, headless bool) (written uint, err error) {
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
		err = s.error(err)
		return
	}

	err = nil

	for lbl := range s.References {
		if lbl[0] == '<' || lbl[0] == '>' {
			// TODO: handle self-ref #>* and #<*
			var num uint16
			var ok bool

			num, ok = s.Constants[lbl[1:]]
			if !ok {
				num, ok = s.Labels[lbl[1:]]
				if !ok {
					err = s.errorf("unknown label: %s", lbl)
					return
				}
			}
			for _, ref := range s.References[lbl] {
				value := num + uint16(s.Memory[ref.Address])
				if lbl[0] == '>' {
					// Produce high-byte of expression
					s.Memory[ref.Address] = uint8((value >> 8) & 0xff)
				} else {
					// Produce low-byte of expression
					s.Memory[ref.Address] = uint8((value >> 0) & 0xff)
				}
			}
			continue
		}

		num, ok := s.Constants[lbl]
		if ok {
			for _, ref := range s.References[lbl] {
				if num <= 0xFF {
					s.Memory[ref.Address] += uint8(num)
				} else {
					binary.LittleEndian.PutUint16(s.Memory[ref.Address:], num)
				}
			}
			continue
		}

		if lbl != "*" {
			addr, ok := s.Labels[lbl]
			if !ok {
				err = s.errorf("unknown label: %s", lbl)
				return
			}

			for _, ref := range s.References[lbl] {
				pos := ref.Address
				num := binary.LittleEndian.Uint16(s.Memory[pos:])
				if !ref.Relative {
					binary.LittleEndian.PutUint16(s.Memory[pos:], num+addr)
					continue
				}

				s.Memory[pos] = uint8(addr - (pos + 1))
			}
		} else {
			// self reference
			for _, ref := range s.References[lbl] {
				if ref.Relative {
					pos := int(ref.Address)
					addr := pos - 1
					offset := int(s.Memory[pos]) - 1 // needed for Branch ops
					s.Memory[pos] = uint8(addr - pos + offset)
					continue
				}
				pos := ref.Address
				addr := pos - 1
				num := binary.LittleEndian.Uint16(s.Memory[pos:])
				binary.LittleEndian.PutUint16(s.Memory[pos:], num+addr)
			}
		}
	}

	for _, chk := range s.Checkpoints {
		var xor uint8
		for _, b := range s.Memory[s.Origin:chk] {
			xor ^= b
		}
		s.Memory[chk] = xor
	}

	written = uint(s.Written)
	if !headless {
		if err = binary.Write(dst, binary.LittleEndian, s.Origin); err != nil {
			return
		}
		written += 2

		if err = binary.Write(dst, binary.LittleEndian, s.Written); err != nil {
			return
		}
		written += 2
	}

	dst.Write(s.Memory[s.Origin:s.Address])

	return
}

// address is a location in the 6502's main memory.
type address = uint16

type state struct {
	Reader       *bufio.Reader
	Labels       map[string]address
	CurrentLabel string
	Constants    map[string]uint16
	References   map[string][]*reference
	Checkpoints  []address

	Memory  [0xFFFF]byte
	Origin  address
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
	implied
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

	if i < len(line) {
		if line[i] == ' ' || line[i] == '\t' {
			i++
		}
	}

	remaining = line[i:]
	return
}

// readNumber parses text for a number (decimal, hex, or binary) and returns
// the uint16 representation of it along with the remaining text. If parsing
// fails, an error is returned and the other returned values should be ignored.
func readNumber(text []byte) (uint16, []byte, error) {
	if text[0] == '$' {
		// Read hex literal.
		var i int
		for i = 1; i < len(text); i++ {
			if !isHex(text[i]) {
				break
			}
		}

		num, err := strconv.ParseUint(string(text[1:i]), 16, 16)
		if err != nil {
			return 0, text, err
		}

		return uint16(num), text[i:], err
	}

	if text[0] == '%' {
		// Read binary literal.
		var i int
		for i = 1; i < len(text); i++ {
			if text[i] != '0' && text[i] != '1' {
				break
			}
		}

		num, err := strconv.ParseUint(string(text[1:i]), 2, 16)
		if err != nil {
			return 0, text, err
		}

		return uint16(num), text[i:], err
	}

	if isDigit(text[0]) {
		var i int
		for i = 0; i < len(text); i++ {
			if !isDigit(text[i]) {
				break
			}
		}

		num, err := strconv.ParseUint(string(text[0:i]), 10, 16)
		if err != nil {
			return 0, text, err
		}

		return uint16(num), text[i:], err
	}

	if len(text) >= 3 {
		// ASCII character
		if text[0] == '\'' && text[2] == '\'' {
			// low-ASCII (high-bit off)
			return uint16(text[1]), text[3:], nil
		}
		if text[0] == '"' && text[2] == '"' {
			// high-ASCII (high-bit on)
			return uint16(text[1] | highASCII), text[3:], nil
		}
	}

	return 0, text, fmt.Errorf("expected hex, binary, or decimal literal; got %s", text)
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

	if len(s.Line) == 0 {
		// Skip empty lines.
		return
	}

	trimmed := string(s.Line)
	trimmed = strings.Trim(trimmed, " \t")
	if len(trimmed) == 0 || trimmed[0] == '*' || trimmed[0] == ';' {
		// Skip comments.
		return
	}

	line := s.Line

	var label string
	label, line = readLabel(line)

	// Note the address of the label, if there is one.
	if label != "" {
		if label[0] != '.' && label[0] != ':' {
			s.CurrentLabel = label
		} else {
			// Local Label
			label = s.CurrentLabel + label
		}
		s.Labels[label] = s.Address
	}

	var mneumonic string
	mneumonic, line = readMneumonic(line)

	switch mneumonic {
	case "ORG":
		s.Address, _, err = readNumber(line)
		s.Origin = s.Address
		return

	case "EQU":
		var def uint16
		var ref string
		def, ref, err = parseOperandValue(line)
		if aliasedValue, ok := s.Constants[ref]; ok {
			def = aliasedValue
		} else if aliasedValue, ok := s.Labels[ref]; ok {
			def = aliasedValue
		}
		s.Constants[label] = def
		return

	case "CHK":
		s.Checkpoints = append(s.Checkpoints, s.Address)
		s.write(0x00)
		return

	case "DFB":
		refs := bytes.Split(line, []byte{','})

		for _, txt := range refs {
			var num uint16
			var ref string
			num, ref, err = parseOperandValue(txt)

			if ref == "" {
				s.writeShort(num)
				continue
			}

			// num, line, err = readNumber([]bytes(line)
			num, ok := s.Constants[ref]
			if !ok {
				num, _, err = readNumber([]byte(ref))
				if err != nil {
					err = fmt.Errorf("unknown constant: %s", ref)
					return
				}
			}
			s.writeShort(num)
		}

		return

	case "HEX":
		var num uint16
		for i := 0; i+1 < len(line); i += 2 {
			if line[i] == ' ' {
				break
			}
			num, _, err = readNumber(append([]byte{'$'}, line[i:i+2]...))
			if err != nil {
				return
			}
			s.writeShort(num)
		}
		return

	case "ASC":
		if !(line[0] == '\'' || line[0] == '"') {
			err = fmt.Errorf("unexpected character: %c", line[0])
			return
		}

		var mask uint8
		if line[0] == '"' {
			mask = highASCII
		}

		quote := line[0]

		for prev, ch := range line[1:] {
			escaped := line[prev] == '\\'
			if ch == quote && !escaped {
				return
			}
			s.write(ch | mask)
		}

		err = fmt.Errorf("unterminated string")
		return

	case "LST":
		// Legal MERLIN instruction, but no affect on assembly
		return
	}

	// TODO: Consider using two lookup tables (opcode, lengths) instead.
	//  opcode $F2 = Invalid mode
	//  opcode $02 = Ambiguous; could be Absolute or Zero Page
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
		if ref[0] == '.' || ref[0] == ':' {
			// Local label
			ref = s.CurrentLabel + ref
		}

		if def, ok := s.Constants[ref]; ok {
			num += def
		} else if refAddr, ok := s.Labels[ref]; ok {
			num += refAddr
		} else {
			if mode == immediate && ref[0] != '<' && ref[0] != '>' {
				// Handle "LDA #ENTRY" as if it were "LDA #<ENTRY"
				ref = "<" + ref
			}
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
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0xB5)
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
			if num < 0xFF && refAdded == nil {
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

	case "STA":
		switch mode {
		case indexedIndirect:
			s.write(0x81)
			s.writeShort(num)
		case indirectIndex:
			s.write(0x91)
			s.writeShort(num)
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0x95)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0x9D)
			s.writeNumber(num)
		case absoluteY:
			// Absolute,Y
			s.write(0x99)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0x85)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0x8D)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "DEC":
		switch mode {
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0xD6)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0xDE)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0xC6)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0xCE)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "INC":
		switch mode {
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0xF6)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0xFE)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0xE6)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0xEE)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "LDX":
		switch mode {
		case immediate:
			s.write(0xA2)
			s.writeShort(num)
		case absoluteY:
			if num < 0xFF && refAdded == nil {
				s.write(0xB6)
				s.writeShort(num)
				break
			}
			s.write(0xBE)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
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

	case "STX":
		switch mode {
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0x86)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0x8E)
			s.writeNumber(num)

		case absoluteY:
			if num < 0xFF && refAdded == nil {
				s.write(0x96)
				s.writeShort(num)
				break
			}
			fallthrough

		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "LDY":
		switch mode {
		case immediate:
			s.write(0xA0)
			s.writeShort(num)
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				s.write(0xB4)
				s.writeShort(num)
				break
			}
			s.write(0xBC)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0xA4)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0xAC)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "STY":
		switch mode {
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0x84)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0x8C)
			s.writeNumber(num)

		case absoluteX:
			if num < 0xFF && refAdded == nil {
				s.write(0x94)
				s.writeShort(num)
				break
			}
			fallthrough

		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

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

	case "JSR":
		if mode != absolute {
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}
		s.write(0x20)
		s.writeNumber(num)

	case "BIT":
		if mode != absolute {
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}
		if num < 0xFF && refAdded == nil {
			// Zero Page
			s.write(0x24)
			s.writeShort(num)
			break
		}
		// Absolute
		s.write(0x2C)
		s.writeNumber(num)

	case "ADC":
		switch mode {
		case immediate:
			s.write(0x69)
			s.writeShort(num)
		case indexedIndirect:
			s.write(0x61)
			s.writeShort(num)
		case indirectIndex:
			s.write(0x71)
			s.writeShort(num)
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0x75)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0x7D)
			s.writeNumber(num)
		case absoluteY:
			// Absolute,Y
			s.write(0x79)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0x65)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0x6D)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "SBC":
		switch mode {
		case immediate:
			s.write(0xE9)
			s.writeShort(num)
		case indexedIndirect:
			s.write(0xE1)
			s.writeShort(num)
		case indirectIndex:
			s.write(0xF1)
			s.writeShort(num)
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0xF5)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0xFD)
			s.writeNumber(num)
		case absoluteY:
			// Absolute,Y
			s.write(0xF9)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0xE5)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0xED)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "EOR":
		switch mode {
		case immediate:
			s.write(0x49)
			s.writeShort(num)
		case indexedIndirect:
			s.write(0x41)
			s.writeShort(num)
		case indirectIndex:
			s.write(0x51)
			s.writeShort(num)
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0x55)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0x5D)
			s.writeNumber(num)
		case absoluteY:
			// Absolute,Y
			s.write(0x59)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0x45)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0x4D)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "ORA":
		switch mode {
		case immediate:
			s.write(0x09)
			s.writeShort(num)
		case indexedIndirect:
			s.write(0x01)
			s.writeShort(num)
		case indirectIndex:
			s.write(0x11)
			s.writeShort(num)
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0x15)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0x1D)
			s.writeNumber(num)
		case absoluteY:
			// Absolute,Y
			s.write(0x19)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0x05)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0x0D)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "AND":
		switch mode {
		case immediate:
			s.write(0x29)
			s.writeShort(num)
		case indexedIndirect:
			s.write(0x21)
			s.writeShort(num)
		case indirectIndex:
			s.write(0x31)
			s.writeShort(num)
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0x35)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0x3D)
			s.writeNumber(num)
		case absoluteY:
			// Absolute,Y
			s.write(0x39)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0x25)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0x2D)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "CMP":
		switch mode {
		case immediate:
			s.write(0xC9)
			s.writeShort(num)
		case indexedIndirect:
			s.write(0xC1)
			s.writeShort(num)
		case indirectIndex:
			s.write(0xD1)
			s.writeShort(num)
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0xD5)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0xDD)
			s.writeNumber(num)
		case absoluteY:
			// Absolute,Y
			s.write(0xD9)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0xC5)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0xCD)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "CPX":
		switch mode {
		case immediate:
			s.write(0xE0)
			s.writeShort(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0xE4)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0xEC)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "CPY":
		switch mode {
		case immediate:
			s.write(0xC0)
			s.writeShort(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0xC4)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0xCC)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "ASL":
		switch mode {
		case implied:
			s.write(0x0A)
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0x16)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0x1E)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0x06)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0x0E)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "ROL":
		switch mode {
		case implied:
			s.write(0x2A)
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0x36)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0x3E)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0x26)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0x2E)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "LSR":
		switch mode {
		case implied:
			s.write(0x4A)
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0x56)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0x5E)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0x46)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0x4E)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	case "ROR":
		switch mode {
		case implied:
			s.write(0x6A)
		case absoluteX:
			if num < 0xFF && refAdded == nil {
				// Zero Page,X
				s.write(0x76)
				s.writeShort(num)
				break
			}
			// Absolute,X
			s.write(0x7E)
			s.writeNumber(num)
		case absolute:
			if num < 0xFF && refAdded == nil {
				// Zero Page
				s.write(0x66)
				s.writeShort(num)
				break
			}
			// Absolute
			s.write(0x6E)
			s.writeNumber(num)
		default:
			err = fmt.Errorf("invalid mode for %s: %v", mneumonic, mode)
			return
		}

	default:
		goto TRYBRANCH
	}

	return

TRYBRANCH:
	if refAdded != nil {
		refAdded.Relative = true
	} else {
		num -= (s.Address + 2)
	}

	switch mneumonic {
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

	default:
		err = fmt.Errorf(`unknown mneumonic: "%s"`, mneumonic)
		return
	}

	s.writeShort(num)
	return
}

func parseOperand(text []byte) (mode addressingMode, val []byte, err error) {
	var i int

	if len(text) == 0 || text[0] == ' ' {
		mode = implied
		return
	}

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
	if len(val) == 0 {
		return
	}

	end := len(val)
	for i, ch := range val {
		if ch == '-' || ch == '+' || ch == ' ' {
			end = i
			break
		}
	}

	if isLetter(val[0]) || val[0] == '<' || val[0] == '>' {
		ref = string(val[0:end])
	} else if (val[0] == '.' || val[0] == ':') && len(val) > 1 {
		ref = string(val[0:end])
	} else if val[0] == '*' {
		ref = string(val[0])
	} else {
		num, _, err = readNumber(val[0:end])
		if err != nil {
			return
		}
	}

	val = val[end:]

	// According to the Merlin manual, the "assembler supports four arithmetic
	// operations: +, -, /, and *." "All ... operations are done from left to
	// right (2+3*5 would assemble as 25 and not 17)."
	for len(val) > 0 {
		var num2 uint16
		switch val[0] {
		case ' ':
			// Assume the rest of the line is a comment.
			return
		case '+':
			num2, val, err = readNumber(val[1:])
			if err != nil {
				return
			}
			num += num2
		case '-':
			num2, val, err = readNumber(val[1:])
			if err != nil {
				return
			}
			num -= num2
		case '*':
			num2, val, err = readNumber(val[1:])
			if err != nil {
				return
			}
			num *= num2
		case '/':
			num2, val, err = readNumber(val[1:])
			if err != nil {
				return
			}
			num /= num2
		default:
			err = fmt.Errorf("invalid arithmetic operator: %c", val[0])
			return
		}
	}

	return
}

func (s *state) error(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("line %d - %s", s.LineNumber, err.Error())
}

func (s *state) errorf(format string, a ...interface{}) error {
	return s.error(fmt.Errorf(format, a...))
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

/*
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package avxtwo2sve

import (
	"fmt"
	"strconv"
	"strings"
)

func getRegister(reg string, suffix string) string {
	if reg[0] == 'Y' {
		num, err := strconv.Atoi(reg[1:])
		if err != nil {
			panic(err)
		}
		if suffix != "" {
			suffix = "." + suffix // prepend with '.' for proper output
		}
		return fmt.Sprintf("z%d%s", num, suffix)
	}
	panic(fmt.Sprintf("getRegister(%s): unsupported", reg))
}

func getRegisterX(reg string, suffix string) string {
	if reg[0] == 'X' {
		num, err := strconv.Atoi(reg[1:])
		if err != nil {
			panic(err)
		}
		if suffix != "" {
			suffix = "." + suffix // prepend with '.' for proper output
		}
		return fmt.Sprintf("z%d%s", num, suffix)
	}
	panic(fmt.Sprintf("getRegisterX(%s): unsupported", reg))
}

func getImmediate(imm string) int64 {
	if imm[:3] == "$0x" {
		num, err := strconv.ParseInt(imm[3:], 16, 64)
		if err != nil {
			panic(err)
		}
		return num
	}
	panic(fmt.Sprintf("getImmediate(%s): unsupported", imm))
}

func getScalarReg(reg string, armAddrMode bool) string {
	getreg := func(r string) string {
		num := -1
		if len(r) == 2 && r[1] == 'X' {
			num = int(r[0] - 'A')
		} else if r == "SI" {
			num = 4
		} else if r == "DI" {
			num = 5
		} else if r == "BP" {
			num = 6
		} else if len(r) >= 2 && r[0] == 'R' {
			var err error
			if num, err = strconv.Atoi(r[1:]); err != nil {
				panic(err)
			}
		}
		if num == -1 {
			panic("failed to convert scalar register")
		}
		return fmt.Sprintf("%d", num)
	}

	if strings.Contains(reg, "(") && strings.Contains(reg, ")") {
		// addressing mode
		p := strings.Split(reg, "(")
		if len(p) == 2 {
			suffix, offset := "", 0
			if len(p[0]) > 0 {
				var err error
				offset, err = strconv.Atoi(p[0])
				if err != nil {
					panic(err)
				}
				if offset != 0 {
					suffix = fmt.Sprintf(", #%d, MUL VL", offset/32)
				}
			}
			r := strings.ReplaceAll(p[1], ")", "")
			num := getreg(r)
			if armAddrMode {
				return fmt.Sprintf("[x%s%s]", num, suffix)
			} else {
				if offset != 0 {
					return fmt.Sprintf("%d(x%s)", offset, num)
				} else {
					return fmt.Sprintf("(x%s)", num)
				}
			}
		}
	} else {
		num := getreg(reg)
		return fmt.Sprintf("x%s", num)
	}
	panic(fmt.Sprintf("getScalarReg(%s): unsupported", reg))
}

type MnemonicType struct {
	Mnemonic string
	Suffix   string
}

var mnemonicTbl = make(map[string]MnemonicType)

func init() {
	mnemonicTbl["VPSRLQ"] = MnemonicType{"lsr", "d"}
	mnemonicTbl["VPAND"] = MnemonicType{"and", "d"}
	mnemonicTbl["VPSHUFB"] = MnemonicType{"tbl", "b"}
	mnemonicTbl["VPBROADCASTB"] = MnemonicType{"dup", "b"}
	mnemonicTbl["VPXOR"] = MnemonicType{"eor", "d"}
	mnemonicTbl["VMOVDQU"] = MnemonicType{"", ""} // either 'ldr' or 'str'
	mnemonicTbl["ADDQ"] = MnemonicType{"add", ""}
	mnemonicTbl["SHRQ"] = MnemonicType{"lsr", ""}
	mnemonicTbl["TESTQ"] = MnemonicType{"tst", ""}
	mnemonicTbl["DECQ"] = MnemonicType{"subs", ""}
	mnemonicTbl["JZ"] = MnemonicType{"BEQ", ""}
	mnemonicTbl["JNZ"] = MnemonicType{"BNE", ""}
}

func AvxTwo2Sve(avx2 string, patchLabel func(string) string) (out string, plan9 bool, err error) {
	p := strings.Fields(avx2)
	for i := range p {
		if p[i][len(p[i])-1] == ',' { // strip trailing comma
			p[i] = p[i][:len(p[i])-1]
		}
	}
	mnemonic, p := p[0], p[1:]
	sveMT := mnemonicTbl[mnemonic]
	switch mnemonic {
	case "VPSRLQ", "VPAND", "VPSHUFB", "VPXOR", "VPBROADCASTB":
		if len(p) == 3 {
			zd, zn := getRegister(p[2], sveMT.Suffix), getRegister(p[1], sveMT.Suffix)
			if p[0][0] == '$' {
				imm := getImmediate(p[0])
				return fmt.Sprintf("%s %s, %s, #%d", sveMT.Mnemonic, zd, zn, imm), false, nil
			} else if p[0][0] == 'Y' {
				zn2 := getRegister(p[0], sveMT.Suffix)
				return fmt.Sprintf("%s %s, %s, %s", sveMT.Mnemonic, zd, zn, zn2), false, nil
			}
		} else if len(p) == 2 {
			zd, zn := getRegister(p[1], sveMT.Suffix), ""
			if p[0][0] == 'Y' {
				zn = getRegister(p[0], sveMT.Suffix)
			} else {
				zn = getRegisterX(p[0], sveMT.Suffix)
				if sveMT.Mnemonic == "dup" {
					zn += "[0]" //add
				}
			}
			return fmt.Sprintf("%s %s, %s", sveMT.Mnemonic, zd, zn), false, nil
		}

	case "VMOVDQU":
		scaledIndex := func(in string) (xn, xm string) {
			s := strings.Split(in, ")(")
			xn = getScalarReg(s[0]+")", true)
			xm = strings.ReplaceAll(s[1], "*8)", "")
			xm = getScalarReg("("+xm+")", true)
			xn = strings.NewReplacer("[", "", "]", "").Replace(xn)
			xm = strings.NewReplacer("[", "", "]", "").Replace(xm)
			return
		}
		if len(p) == 2 {
			if p[1][0] == 'Y' { // load
				zd := getRegister(p[1], sveMT.Suffix)
				if strings.Contains(p[0], "*8)") {
					xn, xm := scaledIndex(p[0])
					return fmt.Sprintf("ld1d { %s.%s }, p0/z, [%s, %s, lsl #3]", zd, "d", xn, xm), false, nil
				} else {
					xn := getScalarReg(p[0], true)
					return fmt.Sprintf("ldr %s, %s", zd, xn), false, nil
				}
			} else if p[0][0] == 'Y' { // store
				zd := getRegister(p[0], sveMT.Suffix)
				if strings.Contains(p[1], "*8)") {
					xn, xm := scaledIndex(p[1])
					return fmt.Sprintf("st1d { %s.%s }, p0, [%s, %s, lsl #3]", zd, "d", xn, xm), false, nil
				} else {
					xn := getScalarReg(p[1], true)
					return fmt.Sprintf("str %s, %s", zd, xn), false, nil
				}
			}
		}

	case "MOVQ":
		if len(p) == 2 {
			if p[1][0] == 'X' {
				xd := getRegisterX(p[1], "d")
				xn := getScalarReg((p[0]), true)
				return fmt.Sprintf("mov %s, %s", xd, xn), false, nil
			}
			xn := getScalarReg((p[1]), true)
			src := p[0]
			if strings.HasPrefix(src, "$0x") {
				imm := getImmediate(src)
				return fmt.Sprintf("mov %s, #%d", xn, imm), false, nil
			} else if !strings.Contains(src, "(FP)") { // are we not loading from stack?
				src = getScalarReg(src, false)
				src = strings.ReplaceAll(src, "x", "R")
			}
			return fmt.Sprintf("MOVD %s, %s", src, strings.ReplaceAll(xn, "x", "R")), true, nil
		}

	case "ADDQ", "SHRQ":
		if len(p) == 2 {
			xn := getScalarReg((p[1]), true)
			if p[0][0] == '$' {
				imm := getImmediate(p[0])
				return fmt.Sprintf("%s %s, %s, #%d", sveMT.Mnemonic, xn, xn, imm), false, nil
			} else {
				xn2 := getScalarReg(p[0], true)
				return fmt.Sprintf("%s %s, %s, %s", sveMT.Mnemonic, xn, xn, xn2), false, nil
			}
		}

	case "TESTQ":
		if len(p) == 2 {
			xn := getScalarReg((p[1]), true)
			xn2 := getScalarReg(p[0], true)
			return fmt.Sprintf("%s %s, %s", sveMT.Mnemonic, xn, xn2), false, nil
		}

	case "DECQ":
		if len(p) == 1 {
			xn := getScalarReg((p[0]), true)
			return fmt.Sprintf("%s %s, %s, #1", sveMT.Mnemonic, xn, xn), false, nil
		}

	case "VZEROUPPER":
		return "", true, nil

	case "JZ", "JNZ":
		out = strings.ReplaceAll(avx2, mnemonic, sveMT.Mnemonic)
		if patchLabel != nil {
			out = patchLabel(out)
		}
		return out, true, nil

	case "RET":
		return avx2, true, nil
	}

	return "", false, fmt.Errorf("avxTwo2Sve unsupported: %s", avx2)
}

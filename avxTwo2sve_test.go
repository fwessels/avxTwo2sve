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
	"strings"
	"testing"
)

func TestInstructions(t *testing.T) {
	testCases := []struct {
		avx2 string
		sve  string
	}{
		// AVX2
		{"VPSRLQ  $0x04, Y9, Y10", "lsr z10.d, z9.d, #4"},
		{"VPAND   Y4, Y9, Y9    ", "and z9.d, z9.d, z4.d"},
		{"VPSHUFB Y11, Y5, Y7   ", "tbl z7.b, z5.b, z11.b"},
		{"VPBROADCASTB X4, Y4   ", "dup z4.b, z4.b[0]"},
		{"VZEROUPPER            ", ""},
		//
		// loading / storing
		{"VMOVDQU (BX), Y9      ", "ldr z9, [x1]"},
		{"VMOVDQU 32(BX), Y11   ", "ldr z11, [x1, #1, MUL VL]"},
		{"VMOVDQU (R3)(R4*8), Y2", "ld1d { z2.d }, p0/z, [x3, x4, lsl #3]"},
		{"VMOVDQU Y0, (R8)(R9*8)", "st1d { z0.d }, p0, [x8, x9, lsl #3]"},
		{"MOVQ  (DX), BX        ", "MOVD (R3), R1"},
		{"MOVQ  24(DX), SI      ", "MOVD 24(R3), R4"},
		{"MOVQ  72(DX), DX      ", "MOVD 72(R3), R3"},
		{"MOVQ  $0x0000000f, R10", "mov x10, #15"},
		{"MOVQ  R10, X4         ", "mov z4.d, x10"},
		//
		// arithmetic
		{"ADDQ   $0x40, BX     ", "add x1, x1, #64"},
		{"ADDQ   $0x100, R2    ", "add x2, x2, #256"},
		{"ADDQ   R10, R9       ", "add x9, x9, x10"},
		{"SHRQ  $0x06, AX      ", "lsr x0, x0, #6"},
		{"TESTQ AX, AX         ", "tst x0, x0"},
		{"DECQ  AX             ", "subs x0, x0, #1"},
		//
		// loading from stack --> keep plan9 syntax
		{"MOVQ  n+80(FP), AX         ", "MOVD n+80(FP), R0"},
		{"MOVQ  matrix_base+0(FP), CX", "MOVD matrix_base+0(FP), R2"},
	}

	for i, tc := range testCases {
		avx2 := strings.TrimSpace(tc.avx2)
		if sve, _, err := AvxTwo2Sve(avx2, nil); err != nil {
			t.Errorf("TestInstructions(%d): `%s`: got error: %s", i, avx2, err)
		} else if sve != tc.sve {
			t.Errorf("TestInstructions(%d): `%s`: got: `%s` want: `%s`", i, avx2, sve, tc.sve)
		}
	}
}

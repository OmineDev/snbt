package snbt

import (
	_ "embed"
	"math"
	"neomega_nbt/base_io/lflb"
	"neomega_nbt/base_io/lflb/sources"
	"neomega_nbt/snbt/tokens"
	whitespace "neomega_nbt/snbt/tokens/white_space"
	"reflect"
	"testing"
)

func consumeWhiteSpaceAndComma[S lflb.Source](src S) {
	lflb.ReadFinity(src, whitespace.FinityVariyLenWhiteSpace{})
	lflb.ReadFinity(src, tokens.Specific(','))
	lflb.ReadFinity(src, whitespace.FinityVariyLenWhiteSpace{})
}

func assertArr[S lflb.Source, I any](
	src S, val []I, t *testing.T) {
	getV, err := DecodeFrom(src)
	if err != nil {
		t.Logf("fail to decode: %v\n", src)
		t.FailNow()
	}

	if reflect.ValueOf(getV).Len() != len(val) {
		t.Logf("length mismatch: %v!=%v", reflect.ValueOf(getV).Len(), len(val))
		t.FailNow()
	}
	v := reflect.ValueOf(getV)
	for i := 0; i < v.Len(); i++ {
		getV := v.Index(i).Interface()
		wantV := val[i]
		if reflect.TypeOf(wantV).Kind() != reflect.TypeOf(getV).Kind() {
			t.Logf("type mismatch: %v!=%v", wantV, getV)
			t.FailNow()
		}
		if !reflect.DeepEqual(wantV, getV) {
			t.Logf("value mismatch: %v!=%v", wantV, getV)
			t.FailNow()
		}
	}
}

func TestSnbtDecode(t *testing.T) {

	seq := " \t\n\v\f\r  "
	src := sources.NewBytesSourceFromString(seq)
	if v, err := DecodeFrom(src); err != ErrNotSNBT {
		t.Errorf("get v: %v\n", v)
		t.FailNow()
	}

	seq = ""
	src = sources.NewBytesSourceFromString(seq)
	if v, err := DecodeFrom(src); err != ErrNoData || v != nil {
		t.Errorf("get v: %v\n", v)
		t.FailNow()
	}

	seq = "abc"
	src = sources.NewBytesSourceFromString(seq)
	if v, err := DecodeFrom(src); err != nil || v != "abc" {
		t.Errorf("get v: %v\n", v)
		t.FailNow()
	}

	// should be number, not string
	seq = "-123"
	src = sources.NewBytesSourceFromString(seq)
	if v, err := DecodeFrom(src); err != nil || v != int32(-123) {
		t.Errorf("get v: %v\n", v)
		t.FailNow()
	}

	seq = "123_abc"
	src = sources.NewBytesSourceFromString(seq)
	if v, err := DecodeFrom(src); err != nil || v != "123_abc" {
		t.Errorf("get v: %v\n", v)
		t.FailNow()
	}

	// nbtlib will parse "tRUe" as TAG_Byte(1)
	seq = "tRUe"
	src = sources.NewBytesSourceFromString(seq)
	if v, err := DecodeFrom(src); err != nil || v != int8(1) {
		t.Errorf("get v: %v\n", v)
		t.FailNow()
	}

	// nbtlib will parse "FaLSE" as TAG_Byte(0)
	seq = "FaLSE"
	src = sources.NewBytesSourceFromString(seq)
	if v, err := DecodeFrom(src); err != nil || v != int8(0) {
		t.Errorf("get v: %v\n", v)
		t.FailNow()
	}

	// nbtlib will parse this as
	// TAG_ByteArray([TAG_Byte(1), TAG_Byte(2), TAG_Byte(3)])
	seq = `[B;
				1b,
				2B,
				3B
			]`
	src = sources.NewBytesSourceFromString(seq)
	if _, err := DecodeFrom(src); err != nil {
		t.Errorf("error parse nbt number array: %v\n", err)
		t.FailNow()
	}
	src = sources.NewBytesSourceFromString(seq)
	assertArr(src, []int8{1, 2, 3}, t)

	// shoud be []byte{1, 2, 1, 0}
	seq = "[B;1b,2b,true,false]"
	src = sources.NewBytesSourceFromString(seq)
	if _, err := DecodeFrom(src); err != nil {
		t.Errorf("error parse nbt number array: %v\n", err)
		t.FailNow()
	}
	src = sources.NewBytesSourceFromString(seq)
	assertArr(src, []int8{1, 2, 1, 0}, t)

	/*
		下面的内容摘自 Wiki。
		-----
		数值类型标签和字符串标签：
		1. true 和 false将分别被转换为 1b 和 0b。
		2. 对于带有字母后缀(B、S、L、F、D，或者它们的小写字母)的数字，
		   将被解析为相应的数据类型。比如，3s(或3S)将被解析为短整型值，
		   3.2f(或3.2F)将被解析为单精度浮点型值。
		3. 若没有指定尾缀，但带有小数点，则该数字会被直接当做双精度浮点数类型的值
		4. 无小数点的二进制 32 位大小内的数字将被当做整型值。
		5. 除以上情况以外的其他情况，**都将当做字符串类型的值**。
		-----
		下面测试用例的 66666666666 不在 32 位整型范围内，
		因此其不满足上面的前四种情况。
		因此，66666666666 被当作字符串类型的值，而非 int32。
	*/
	seq = "66666666666"
	src = sources.NewBytesSourceFromString(seq)
	if v, err := DecodeFrom(src); err != nil || v != "66666666666" {
		t.Errorf("get v: %v\n", v)
		t.FailNow()
	}

	// number tests
	seq = "-123,123,-1,1,-1b,1b,103b,-104b,-1s,1s,103s,-104s,-1l,1l,103l,-104l,-56.78,3.2,-3.,-.2,-234.,-.456,123E-2,-.456E1,3.E1,45.58E-12,123.456E-8f,123.456E10d,true,false,3b,end"
	src = sources.NewBytesSourceFromString(seq)

	assertNumber := func(val any, T byte) {
		number, err := DecodeFrom(src)
		if err != nil {
			t.Logf("want: %v, get err: %v\n", val, err)
			t.FailNow()
		}
		switch T {
		case 'I':
			getV, ok1 := number.(int32)
			wantV, ok2 := val.(int32)
			if !ok1 || !ok2 {
				t.FailNow()
			}
			if getV != wantV {
				t.Logf("want %v, get %v\n", wantV, getV)
				t.FailNow()
			}
		case 'B':
			getV, ok1 := number.(int8)
			wantV, ok2 := val.(int8)
			if !ok1 || !ok2 {
				t.FailNow()
			}
			if getV != wantV {
				t.Logf("want %v, get %v\n", wantV, getV)
				t.FailNow()
			}

		case 'S':
			getV, ok1 := number.(int16)
			wantV, ok2 := val.(int16)
			if !ok1 || !ok2 {
				t.FailNow()
			}
			if getV != wantV {
				t.Logf("want %v, get %v\n", wantV, getV)
				t.FailNow()
			}

		case 'L':
			getV, ok1 := number.(int64)
			wantV, ok2 := val.(int64)
			if !ok1 || !ok2 {
				t.FailNow()
			}
			if getV != wantV {
				t.Logf("want %v, get %v\n", wantV, getV)
				t.FailNow()
			}
		case 'F':
			getV, ok1 := number.(float32)
			wantV, ok2 := val.(float32)
			if !ok1 || !ok2 {
				t.FailNow()
			}
			if math.Abs(float64(getV-wantV)) > 0.0001 {
				t.Logf("want %v, get %v\n", wantV, getV)
				t.FailNow()
			}
		case 'D':
			getV, ok1 := number.(float64)
			wantV, ok2 := val.(float64)
			if !ok1 || !ok2 {
				t.Logf("want %v, get %v\n", wantV, getV)
				t.FailNow()
			}
			if math.Abs(float64(getV-wantV)) > 1e-14 {
				t.Logf("want %v, get %v\n", wantV, getV)
				t.FailNow()
			}
		}
	}
	assertNumber(int32(-123), 'I')
	readNext := func() {
		lflb.ReadFinity(src, tokens.FinityAny{})
	}
	readNext()
	assertNumber(int32(123), 'I')
	readNext()
	assertNumber(int32(-1), 'I')
	readNext()
	assertNumber(int32(1), 'I')

	readNext()
	assertNumber(int8(-1), 'B')
	readNext()
	assertNumber(int8(1), 'B')
	readNext()
	assertNumber(int8(103), 'B')
	readNext()
	assertNumber(int8(-104), 'B')
	readNext()
	assertNumber(int16(-1), 'S')
	readNext()
	assertNumber(int16(1), 'S')
	readNext()
	assertNumber(int16(103), 'S')
	readNext()
	assertNumber(int16(-104), 'S')
	readNext()
	assertNumber(int64(-1), 'L')
	readNext()
	assertNumber(int64(1), 'L')
	readNext()
	assertNumber(int64(103), 'L')
	readNext()
	assertNumber(int64(-104), 'L')
	readNext()
	assertNumber(float64(-56.78), 'D')
	readNext()
	assertNumber(float64(3.2), 'D')
	readNext()
	assertNumber(float64(-3.0), 'D')
	readNext()
	assertNumber(float64(-0.2), 'D')
	readNext()
	assertNumber(float64(-234.0), 'D')
	readNext()
	assertNumber(float64(-0.456), 'D')
	readNext()
	assertNumber(float64(1.23), 'D')
	readNext()
	assertNumber(float64(-4.56), 'D')
	readNext()
	assertNumber(float64(30), 'D')
	readNext()
	assertNumber(float64(45.58e-12), 'D')
	readNext()
	assertNumber(float32(123.456e-8), 'F')
	readNext()
	assertNumber(float64(123.456e10), 'D')
	readNext()
	assertNumber(int8(1), 'B')
	readNext()
	assertNumber(int8(0), 'B')
	readNext()
	assertNumber(int8(3), 'B')
	readNext()
	if v, _ := src.This(); v != 'e' {
		t.FailNow()
	}

	seq = `"abc()  _+:'\"\\测试" "abc()  _+:'\"\\测试`

	assertVal := func(val any) {
		if v, err := DecodeFrom(src); err != nil || v != val {
			t.Logf("want: %v, get v: %v\n", val, v)
			t.FailNow()
		}
	}
	assertFail := func() {
		_, err := DecodeFrom(src)
		if err == nil {
			t.FailNow()
		}
	}

	src = sources.NewBytesSourceFromString(seq)
	assertVal(`abc()  _+:'"\测试`)
	consumeWhiteSpaceAndComma(src)
	lflb.ReadFinity(src, tokens.Specific('\''))
	assertFail()
	if v, _ := src.This(); v != 'a' {
		t.FailNow()
	}

	seq = `'abc()  _+:"\'\\测试' "abc()  _+:'\"\\测试" abc_123.+-ABC, `
	src = sources.NewBytesSourceFromString(seq)
	assertVal(`abc()  _+:"'\测试`)
	consumeWhiteSpaceAndComma(src)
	assertVal(`abc()  _+:'"\测试`)
	consumeWhiteSpaceAndComma(src)
	assertVal(`abc_123.+-ABC`)

	seq = `'abc()  _+:"\'\\测试 `
	src = sources.NewBytesSourceFromString(seq)
	assertFail()
	if v, _ := src.This(); v != 'a' {
		t.FailNow()
	}

	seq = `"abc()  _+:'\"\\测试 `
	src = sources.NewBytesSourceFromString(seq)
	assertFail()
	if v, _ := src.This(); v != 'a' {
		t.FailNow()
	}

	seq = `abc_123.+-ABC`
	src = sources.NewBytesSourceFromString(seq)
	assertVal(`abc_123.+-ABC`)

	seq = "[I; true,false   ] "
	src = sources.NewBytesSourceFromString(seq)
	assertArr(src, []int32{1, 0}, t)

	seq = "[B; true,false   ] "
	src = sources.NewBytesSourceFromString(seq)
	assertArr(src, []int8{1, 0}, t)

	seq = "[I; 12, -34 ,-567, 89,10,11 , -12 ,-13 ,14 ,15   ] "
	src = sources.NewBytesSourceFromString(seq)
	assertArr(src, []int32{12, -34, -567, 89, 10, 11, -12, -13, 14, 15}, t)

	seq = "[B; 12, -34 ,-57, 89,10,11 , -12 ,-13 ,14 ,15   ] "
	src = sources.NewBytesSourceFromString(seq)
	assertArr(src, []int8{12, -34, -57, 89, 10, 11, -12, -13, 14, 15}, t)

	seq = "[L; 12, -34 ,-567, 89,10,11 , -12 ,-13 ,14 ,15   ] "
	src = sources.NewBytesSourceFromString(seq)
	assertArr(src, []int64{12, -34, -567, 89, 10, 11, -12, -13, 14, 15}, t)
	seq = "[I;12, -34 ,-567, 89,10,11 , -12 ,-13 ,14 ,15   "
	src = sources.NewBytesSourceFromString(seq)
	assertFail()
	if v, _ := src.This(); v != '1' {
		t.FailNow()
	}
	seq = "[I;a,12, -34 ,-567, 89,10,11 , -12 ,-13 ,14 ,15]   "
	src = sources.NewBytesSourceFromString(seq)
	assertFail()
	if v, _ := src.This(); v != 'a' {
		t.FailNow()
	}

	seq = "[I;12, --34] "
	src = sources.NewBytesSourceFromString(seq)
	assertFail()
	if v, _ := src.This(); v != '1' {
		t.FailNow()
	}

	seq = "[I;12,,3] "
	src = sources.NewBytesSourceFromString(seq)
	assertFail()
	if v, _ := src.This(); v != '1' {
		t.FailNow()
	}

	seq = "[I;12,3,] "
	src = sources.NewBytesSourceFromString(seq)
	assertFail()
	if v, _ := src.This(); v != '1' {
		t.FailNow()
	}

	seq = "[ 12, -34 ,-567, 89,10,11 , -12 ,-13 ,14 ,15   ] "
	src = sources.NewBytesSourceFromString(seq)
	assertArr(src, []int32{12, -34, -567, 89, 10, 11, -12, -13, 14, 15}, t)

	seq = "[']',']'] "
	src = sources.NewBytesSourceFromString(seq)
	assertArr(src, []any{"]", "]"}, t)

	// not allowed in snbt, but we allow this
	seq = "[ 12, 123_abc, -12.34E3f ,'bcd',[a,b,bc],{a:1,b:1,c:2}] "
	src = sources.NewBytesSourceFromString(seq)
	assertArr(src, []any{int32(12), "123_abc", float32(-12.34e3), "bcd", []any{"a", "b", "bc"}, map[string]any{
		"a": int32(1), "b": int32(1), "c": int32(2),
	}}, t)

	seq = "{123_abc:{a:1,b:1,c:2},\"嗨 我的世界wiki\": 123, '@{}<>!-=()[]*&^%$#/+`~.\";': 123}"
	src = sources.NewBytesSourceFromString(seq)
	if v, err := DecodeFrom(src); err != nil {
		t.Logf("get err: %v\n", err)
		t.FailNow()
	} else {
		if !reflect.DeepEqual(map[string]any{
			"嗨 我的世界wiki":                 int32(123),
			"@{}<>!-=()[]*&^%$#/+`~.\";": int32(123),
			"123_abc": map[string]any{
				"a": int32(1), "b": int32(1), "c": int32(2),
			},
		}, v) {
			t.FailNow()
		}
	}
}

// copied from Tnze-mc/snbt
//
//go:embed test_data/bigTest_test.snbt
var bigTestSNBT string

func BenchmarkSNBT_bigTest(b *testing.B) {
	// BenchmarkSNBT_bigTest-8   	   38328	     28354 ns/op	    7683 B/op	      99 allocs/op
	// speed: 29204 ns/op
	// memory: allocation 13858 B/op 101 allocs/op
	src := sources.NewBytesSourceFromString(bigTestSNBT)
	for i := 0; i < b.N; i++ {
		src.Reset()
		if _, err := DecodeFrom(src); err != nil {
			b.Errorf("decode fail: %v", err)
		}
	}
}

//go:embed test_data/1-dimension_codec.snbt
var dim1SNBT string

func BenchmarkSNBT_1dim(b *testing.B) {
	// BenchmarkSNBT_1dim-8   	    2476	    443519 ns/op	  184238 B/op	    4928 allocs/op
	src := sources.NewBytesSourceFromString(dim1SNBT)
	for i := 0; i < b.N; i++ {
		src.Reset()
		if _, err := DecodeFrom(src); err != nil {
			b.Errorf("decode fail: %v", err)
		}
	}
}

//go:embed test_data/58f6356e-b30c-4811-8bfc-d72a9ee99e73.dat.snbt
var dataSNBT string

func BenchmarkSNBT_data(b *testing.B) {
	// BenchmarkSNBT_data-8   	   36336	     29726 ns/op	   16523 B/op	     322 allocs/op
	src := sources.NewBytesSourceFromString(dataSNBT)
	for i := 0; i < b.N; i++ {
		src.Reset()
		if _, err := DecodeFrom(src); err != nil {
			b.Errorf("decode fail: %v", err)
		}
	}
}

//go:embed test_data/level.dat.snbt
var levelSNBT string

func BenchmarkSNBT_level(b *testing.B) {
	// BenchmarkSNBT_level-8   	   16491	     69444 ns/op	   37689 B/op	     715 allocs/op
	src := sources.NewBytesSourceFromString(levelSNBT)
	for i := 0; i < b.N; i++ {
		src.Reset()
		if _, err := DecodeFrom(src); err != nil {
			b.Errorf("decode fail: %v", err)
		}
	}
}

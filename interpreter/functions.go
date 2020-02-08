package glisp

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"encoding/binary"
	"strconv"
)

var WrongNargs error = errors.New("wrong number of arguments")

type GlispFunction []Instruction
type GlispUserFunction func(*Glisp, string, []Sexp) (Sexp, error)

func CompareFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 2 {
		return SexpNull, WrongNargs
	}

	res, err := Compare(args[0], args[1])
	if err != nil {
		return SexpNull, err
	}

	cond := false
	switch name {
	case "<":
		cond = res < 0
	case ">":
		cond = res > 0
	case "<=":
		cond = res <= 0
	case ">=":
		cond = res >= 0
	case "=":
		cond = res == 0
	case "not=":
		cond = res != 0
	}

	return SexpBool(cond), nil
}

func BinaryIntFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 2 {
		return SexpNull, WrongNargs
	}

	var op IntegerOp
	switch name {
	case "sll":
		op = ShiftLeft
	case "sra":
		op = ShiftRightArith
	case "srl":
		op = ShiftRightLog
	case "mod":
		op = Modulo
	}

	return IntegerDo(op, args[0], args[1])
}

func BitwiseFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 2 {
		return SexpNull, WrongNargs
	}

	var op IntegerOp
	switch name {
	case "bit-and":
		op = BitAnd
	case "bit-or":
		op = BitOr
	case "bit-xor":
		op = BitXor
	}

	accum := args[0]
	var err error

	for _, expr := range args[1:] {
		accum, err = IntegerDo(op, accum, expr)
		if err != nil {
			return SexpNull, err
		}
	}
	return accum, nil
}

func ComplementFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}

	switch t := args[0].(type) {
	case SexpInt:
		return ^t, nil
	case SexpChar:
		return ^t, nil
	}

	return SexpNull, errors.New("Argument to bit-not should be integer")
}

func NumericFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) < 1 {
		return SexpNull, WrongNargs
	}

	var err error
	accum := args[0]
	var op NumericOp
	switch name {
	case "+":
		op = Add
	case "-":
		op = Sub
	case "*":
		op = Mult
	case "/":
		op = Div
	}

	for _, expr := range args[1:] {
		accum, err = NumericDo(op, accum, expr)
		if err != nil {
			return SexpNull, err
		}
	}
	return accum, nil
}

func ConsFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 2 {
		return SexpNull, WrongNargs
	}

	return Cons(args[0], args[1]), nil
}

func FirstFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}

	switch expr := args[0].(type) {
	case SexpPair:
		return expr.head, nil
	case SexpArray:
		return expr[0], nil
	}

	return SexpNull, WrongType
}

func RestFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}

	switch expr := args[0].(type) {
	case SexpPair:
		return expr.tail, nil
	case SexpArray:
		if len(expr) == 0 {
			return expr, nil
		}
		return expr[1:], nil
	case SexpSentinel:
		if expr == SexpNull {
			return SexpNull, nil
		}
	}

	return SexpNull, WrongType
}

func ArrayAccessFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) < 2 {
		return SexpNull, WrongNargs
	}

	var arr SexpArray
	switch t := args[0].(type) {
	case SexpArray:
		arr = t
	default:
		return SexpNull, errors.New("First argument of aget must be array")
	}

	var i int
	switch t := args[1].(type) {
	case SexpInt:
		i = int(t)
	case SexpChar:
		i = int(t)
	default:
		return SexpNull, errors.New("Second argument of aget must be integer")
	}

	if i < 0 || i >= len(arr) {
		return SexpNull, errors.New("Array index out of bounds")
	}

	if name == "aget" {
		return arr[i], nil
	}

	if len(args) != 3 {
		return SexpNull, WrongNargs
	}
	arr[i] = args[2]

	return SexpNull, nil
}

func SgetFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 2 {
		return SexpNull, WrongNargs
	}

	var str SexpStr
	switch t := args[0].(type) {
	case SexpStr:
		str = t
	default:
		return SexpNull, errors.New("First argument of sget must be string")
	}

	var i int
	switch t := args[1].(type) {
	case SexpInt:
		i = int(t)
	case SexpChar:
		i = int(t)
	default:
		return SexpNull, errors.New("Second argument of sget must be integer")
	}

	return SexpChar(str[i]), nil
}

func HashAccessFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) < 2 || len(args) > 3 {
		return SexpNull, WrongNargs
	}

	var hash SexpHash
	switch e := args[0].(type) {
	case SexpHash:
		hash = e
	default:
		return SexpNull, errors.New("first argument of hget must be hash")
	}

	switch name {
	case "hget":
		if len(args) == 3 {
			return hash.HashGetDefault(args[1], args[2])
		}
		return hash.HashGet(args[1])
	case "hset!":
		err := hash.HashSet(args[1], args[2])
		return SexpNull, err
	case "hdel!":
		if len(args) != 2 {
			return SexpNull, WrongNargs
		}
		err := hash.HashDelete(args[1])
		return SexpNull, err
	}

	return SexpNull, nil
}

func SliceFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 3 {
		return SexpNull, WrongNargs
	}

	var start int
	var end int
	switch t := args[1].(type) {
	case SexpInt:
		start = int(t)
	case SexpChar:
		start = int(t)
	default:
		return SexpNull, errors.New("Second argument of slice must be integer")
	}

	switch t := args[2].(type) {
	case SexpInt:
		end = int(t)
	case SexpChar:
		end = int(t)
	default:
		return SexpNull, errors.New("Third argument of slice must be integer")
	}

	switch t := args[0].(type) {
	case SexpArray:
		return SexpArray(t[start:end]), nil
	case SexpStr:
		return SexpStr(t[start:end]), nil
	case SexpData:
		return SexpData(t[start:end]), nil
	}

	return SexpNull, errors.New("First argument of slice must be of type - array, string, data")
}

func LenFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}

	switch t := args[0].(type) {
	case SexpArray:
		return SexpInt(len(t)), nil
	case SexpStr:
		return SexpInt(len(t)), nil
	case SexpData:
		return SexpInt(len(t)), nil
	case SexpHash:
		return SexpInt(HashCountKeys(t)), nil
	}

	return SexpInt(0), errors.New("argument must be string or array")
}

func AppendFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) < 2 {
		return SexpNull, WrongNargs
	}

	coalesce := name[0] == '?'

	var err error

	switch t := args[0].(type) {
	case SexpArray:
		if coalesce {
			for _, arg := range args {
				if IsEmpty(arg) {
					continue
				}
				t = append(t, arg)
			}
			return SexpArray(t), nil
		} else {
			return SexpArray(append(t, args[1:]...)), nil
		}
	case SexpStr:
		for _, arg := range args {
			if coalesce && IsEmpty(arg) {
				continue
			}
			t, err = AppendStr(t, arg)
			if err != nil {
				return nil, err
			}
		}
		return t, nil
	case SexpData:
		return MakeDataFunction(env, name, args)
	}

	return SexpNull, errors.New("First argument of append must be array or string or data")
}

func ConcatFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) < 2 {
		return SexpNull, WrongNargs
	}

	coalesce := name[0] == '?'


	var err error

	switch t := args[0].(type) {
	case SexpArray:
		for _, arg := range args[1:] {
			if coalesce && IsEmpty(arg) {
				continue
			}
			t, err = ConcatArray(t, arg)
			if err != nil {
				return nil, err
			}
		}	
		return t, nil
	case SexpStr:
		for _, arg := range args[1:] {
			if coalesce && IsEmpty(arg) {
				continue
			}
			t, err = ConcatStr(t, arg)
			if err != nil {
				return nil, err
			}
		}
		return t, nil
	case SexpPair:
		var ot Sexp
		for _, arg := range args[1:] {
			if coalesce && IsEmpty(arg) {
				continue
			}
			ot, err = ConcatList(t, arg)
			if err != nil {
				return nil, err
			}
			t = ot.(SexpPair)
		}
		return t, nil
	case SexpData:
		return MakeDataFunction(env, name, args)
	}


	return SexpNull, fmt.Errorf("expected string|data|array|pair got %T", args[0])
}

func ReadFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}
	str := ""
	switch t := args[0].(type) {
	case SexpStr:
		str = string(t)
	default:
		return SexpNull, WrongType
	}
	lexer := NewLexerFromStream(bytes.NewBuffer([]byte(str)))
	parser := Parser{lexer, env}
	return ParseExpression(&parser)
}

func EvalFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}
	newenv := env.Duplicate()
	err := newenv.LoadExpressions(args)
	if err != nil {
		return SexpNull, errors.New("failed to compile expression")
	}
	newenv.pc = 0
	return newenv.Run()
}

func TypeQueryFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}

	var result bool

	switch name {
	case "list?":
		result = IsList(args[0])
	case "pair?":
		result = IsPair(args[0])
	case "null?":
		result = (args[0] == SexpNull)
	case "array?":
		result = IsArray(args[0])
	case "number?":
		result = IsNumber(args[0])
	case "float?":
		result = IsFloat(args[0])
	case "int?":
		result = IsInt(args[0])
	case "char?":
		result = IsChar(args[0])
	case "symbol?":
		result = IsSymbol(args[0])
	case "string?":
		result = IsString(args[0])
	case "hash?":
		result = IsHash(args[0])
	case "data?":
		result = IsData(args[0])
	case "zero?":
		result = IsZero(args[0])
	case "empty?":
		result = IsEmpty(args[0])
	}

	return SexpBool(result), nil
}

func PrintFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) < 1 {
		return SexpNull, WrongNargs
	}

	var str string


	for _, arg := range args {
	switch expr := arg.(type) {
		case SexpStr:
			str = string(expr)
		default:
			str = expr.SexpString()
		}

		switch name {
		case "println":
			fmt.Println(str)
		case "print":
			fmt.Print(str)
		}
	}

	return SexpNull, nil
}

func NotFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}

	result := SexpBool(!IsTruthy(args[0]))
	return result, nil
}

func ApplyFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 2 {
		return SexpNull, WrongNargs
	}
	var fun SexpFunction
	var funargs SexpArray

	switch e := args[0].(type) {
	case SexpFunction:
		fun = e
	default:
		return SexpNull, errors.New("first argument must be function")
	}

	switch e := args[1].(type) {
	case SexpArray:
		funargs = e
	case SexpPair:
		var err error
		funargs, err = ListToArray(e)
		if err != nil {
			return SexpNull, err
		}
	case SexpSentinel:
		funargs = []Sexp{}
	default:
		return SexpNull, errors.New("second argument must be array or list")
	}

	return env.Apply(fun, funargs)
}

func FoldlData(env *Glisp, fun SexpFunction, data SexpData, acc Sexp, sz int) (Sexp, error) {
	var err error

	walk := []byte(data)

	chunks := len(walk)/sz

	for i := 0; i < chunks; i++ {
		acc, err = env.Apply(fun, []Sexp{SexpData(walk[i*sz:i*sz+sz]), acc})
		if err != nil {
			return acc, err
		}
	}

	if len(walk) > chunks * sz {
		remain := len(walk) - chunks * sz

		acc, err = env.Apply(fun, []Sexp{SexpData(walk[chunks*sz:chunks*sz+remain]), acc})
		if err != nil {
			return acc, err
		}
	}

	return acc, nil
}

func FoldLFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) < 3 {
		return SexpNull, WrongNargs
	}
	var fun SexpFunction

	switch e := args[1].(type) {
	case SexpFunction:
		fun = e
	default:
		return SexpNull, fmt.Errorf("first argument must be function had type `%T` val %v", e, e)
	}

	acc := args[2]

	switch e := args[0].(type) {
	case SexpArray:
		return FoldlArray(env, fun, e, acc)
	case SexpPair:
		return FoldlPair(env, fun, e, acc)
	case SexpHash:
		return FoldlHash(env, fun, e, acc)
	case SexpData:
		chunkSz := 1
		if len(args) > 3 {
			if sz, ok := args[3].(SexpInt); ok && int(sz) > 0 {
				chunkSz = int(sz)
			}
		}
		return FoldlData(env, fun, e, acc, chunkSz)
	}

	return SexpNull, fmt.Errorf("second argument must be pair, array, list, hash, or data, had type `%T` val %v", args[1], args[1])
}

func MapFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 2 {
		return SexpNull, WrongNargs
	}
	var fun SexpFunction

	switch e := args[0].(type) {
	case SexpFunction:
		fun = e
	default:
		return SexpNull, fmt.Errorf("first argument must be function had type `%T` val %v", e, e)
	}

	switch e := args[1].(type) {
	case SexpArray:
		return MapArray(env, fun, e)
	case SexpPair:
		return MapList(env, fun, e)
	case SexpHash:
		return MapHash(env, fun, e)
	}
	return SexpNull, fmt.Errorf("second argument must be array, list or hash, had type `%T` val %v", args[1], args[1])
}


func makeData(i *int, data *bytes.Buffer, thing Sexp) error {
	var err error

	switch t := thing.(type) {
	case SexpArray:
		for _, v := range t {
			err = makeData(i, data, v)
			if err != nil {
				return err
			}
			*i++
		}
	case SexpPair:
		err = makeData(i, data, t.head)
		if err != nil {
			return err
		}
		*i++
		err = makeData(i, data, t.tail)
		if err != nil {
			return err
		}
		*i++
	case SexpStr:
		data.WriteString(string(t))
		*i++
	case SexpData:
		data.Write([]byte(t))
		*i++
	case SexpInt:
		binary.Write(data, binary.LittleEndian, int64(int(t)))
		*i++
	case SexpFloat:
		binary.Write(data, binary.LittleEndian, float64(t))
		*i++
	case SexpBool:
		if bool(t) {
			data.WriteByte(1)
		} else {
			data.WriteByte(0)
		}
		*i++
	case SexpChar:
		data.WriteRune(rune(t))
		*i++
	default:
		return fmt.Errorf("MakeData failed for item %v didn't know how to deal with %T type, %v data", thing, thing)
	}
	return nil
}

func MakeDataFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	data := &bytes.Buffer{}
	i := 0

	coalesce := name[0] == '?'

	for _, v := range args {
		if coalesce && IsEmpty(v) {
			continue
		}
		if err := makeData(&i, data, v); err != nil {
			return SexpNull, err
		}
	}
	return SexpData(data.Bytes()), nil
}

func MakeArrayFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) < 1 {
		return SexpNull, WrongNargs
	}

	var size int
	switch e := args[0].(type) {
	case SexpInt:
		size = int(e)
	default:
		return SexpNull, errors.New("first argument must be integer")
	}

	var fill Sexp
	if len(args) == 2 {
		fill = args[1]
	} else {
		fill = SexpNull
	}

	arr := make([]Sexp, size)
	for i := range arr {
		arr[i] = fill
	}

	return SexpArray(arr), nil
}

func ConstructorFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	switch name {
	case "array":
		return SexpArray(args), nil
	case "list":
		return MakeList(args), nil
	case "hash":
		return MakeHash(args, "hash")
	}
	return SexpNull, errors.New("invalid constructor")
}

func SymnumFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}

	switch t := args[0].(type) {
	case SexpSymbol:
		return SexpInt(t.number), nil
	}
	return SexpNull, errors.New("argument must be symbol")
}

func SourceFileFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) < 1 {
		return SexpNull, WrongNargs
	}

	var sourceItem func(item Sexp) error

	sourceItem = func(item Sexp) error {
		switch t := item.(type) {
		case SexpArray:
			for _, v := range t {
				if err := sourceItem(v); err != nil {
					return err
				}
			}
		case SexpPair:
			expr := item
			for expr != SexpNull {
				list := expr.(SexpPair)
				if err := sourceItem(list.head); err != nil {
					return err
				}
				expr = list.tail
			}
		case SexpStr:
			var f *os.File
			var err error

			if f, err = os.Open(string(t)); err != nil {
				return err
			}

			if err = env.SourceFile(f); err != nil {
				return err
			}

			f.Close()
		default:
			return fmt.Errorf("%v: Expected `string`, `list`, `array` given type %T val %v", name, item, item)
		}

		return nil
	}

	for _, v := range args {
		if err := sourceItem(v); err != nil {
			return SexpNull, err
		}
	}

	return SexpNull, nil
}

var MissingFunction = SexpFunction{"__missing", true, 0, false, nil, nil, nil}

func MakeFunction(name string, nargs int, varargs bool,
	fun GlispFunction) SexpFunction {
	var sfun SexpFunction
	sfun.name = name
	sfun.user = false
	sfun.nargs = nargs
	sfun.varargs = varargs
	sfun.fun = fun
	return sfun
}

func MakeUserFunction(name string, ufun GlispUserFunction) SexpFunction {
	var sfun SexpFunction
	sfun.name = name
	sfun.user = true
	sfun.userfun = ufun
	return sfun
}

var BuiltinFunctions = map[string]GlispUserFunction{
	"<":          CompareFunction,
	">":          CompareFunction,
	"<=":         CompareFunction,
	">=":         CompareFunction,
	"=":          CompareFunction,
	"not=":       CompareFunction,
	"sll":        BinaryIntFunction,
	"sra":        BinaryIntFunction,
	"srl":        BinaryIntFunction,
	"mod":        BinaryIntFunction,
	"+":          NumericFunction,
	"-":          NumericFunction,
	"*":          NumericFunction,
	"/":          NumericFunction,
	"bit-and":    BitwiseFunction,
	"bit-or":     BitwiseFunction,
	"bit-xor":    BitwiseFunction,
	"bit-not":    ComplementFunction,
	"read":       ReadFunction,
	"cons":       ConsFunction,
	"first":      FirstFunction,
	"rest":       RestFunction,
	"car":        FirstFunction,
	"cdr":        RestFunction,
	"list?":      TypeQueryFunction,
	"null?":      TypeQueryFunction,
	"array?":     TypeQueryFunction,
	"hash?":      TypeQueryFunction,
	"number?":    TypeQueryFunction,
	"int?":       TypeQueryFunction,
	"float?":     TypeQueryFunction,
	"char?":      TypeQueryFunction,
	"symbol?":    TypeQueryFunction,
	"string?":    TypeQueryFunction,
	"zero?":      TypeQueryFunction,
	"empty?":     TypeQueryFunction,
	"pair?":      TypeQueryFunction,
	"data?":      TypeQueryFunction,
	"println":    PrintFunction,
	"print":      PrintFunction,
	"not":        NotFunction,
	"apply":      ApplyFunction,
	"map":        MapFunction,
	"foldl":      FoldLFunction,
	"make-array": MakeArrayFunction,
	"make-data":  MakeDataFunction,
	"aget":       ArrayAccessFunction,
	"aset!":      ArrayAccessFunction,
	"sget":       SgetFunction,
	"hget":       HashAccessFunction,
	"hset!":      HashAccessFunction,
	"hdel!":      HashAccessFunction,
	"slice":      SliceFunction,
	"len":        LenFunction,
	"append":     AppendFunction,
	"?append":    AppendFunction,
	"concat":     ConcatFunction,
	"?concat":    ConcatFunction,
	"array":      ConstructorFunction,
	"list":       ConstructorFunction,
	"hash":       ConstructorFunction,
	"symnum":     SymnumFunction,
	"str":        StringifyFunction,
	"cvert-str":    ConvertFunction,
	"cvert-int64":  ConvertFunction,
	"cvert-int32":  ConvertFunction,
	"cvert-float32":  ConvertFunction,
	"cvert-float64":  ConvertFunction,
}

func ConvertFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) < 1 {
		return SexpNull, WrongNargs
	}

	switch name {
		case "cvert-str": {
			buffer := &bytes.Buffer{}
			for _, arg := range args {
				switch t := arg.(type) {
					case SexpData:
						buffer.WriteString(string([]byte(t)))
					break;
					default:
						buffer.WriteString(arg.SexpString())
				}
			}
			return SexpStr(buffer.String()), nil
		}
		case "cvert-int64": {
			var ret []Sexp
			for i, arg := range args {
				switch t := arg.(type) {
					case SexpData: {
						buffer := bytes.NewBuffer([]byte(t))
						var value int64
						err := binary.Read(buffer, binary.LittleEndian, &value)
						if err != nil {
							return SexpNull, fmt.Errorf("%T: failed converting %v arg into int; %v", arg, i, err)
						}
						ret = append(ret, SexpInt(int(value)))
					}
					case SexpStr: {
						val, err := strconv.Atoi(string(t))
						if err != nil {
							return SexpNull, fmt.Errorf("%T: failed converting %v arg into int; %v", arg, i, err)
						}
						ret = append(ret, SexpInt(val))
					}
					case SexpFloat: {
						ret = append(ret, SexpInt(int(float64(t))))
					}
					case SexpBool: {
						if bool(t) {
							ret = append(ret, SexpInt(1))
						} else {
							ret = append(ret, SexpInt(0))
						}
					}
					default:
						return SexpNull, fmt.Errorf("%v unable to convert arg %v into int; unimplemented", name, i)
				}
			}
			return SexpArray(ret), nil
		}
		case "cvert-int32": {
			var ret []Sexp
			for i, arg := range args {
				switch t := arg.(type) {
					case SexpData: {
						buffer := bytes.NewBuffer([]byte(t))
						var value int32
						err := binary.Read(buffer, binary.LittleEndian, &value)
						if err != nil {
							return SexpNull, fmt.Errorf("%T: failed converting %v arg into int; %v", arg, i, err)
						}
						ret = append(ret, SexpInt(int(value)))
					}
					case SexpStr: {
						val, err := strconv.Atoi(string(t))
						if err != nil {
							return SexpNull, fmt.Errorf("%T: failed converting %v arg into int; %v", arg, i, err)
						}
						ret = append(ret, SexpInt(val))
					}
					case SexpFloat: {
						ret = append(ret, SexpInt(int(float64(t))))
					}
					case SexpBool: {
						if bool(t) {
							ret = append(ret, SexpInt(1))
						} else {
							ret = append(ret, SexpInt(0))
						}
					}
					default:
						return SexpNull, fmt.Errorf("%v unable to convert arg %v into int; unimplemented", name, i)
				}
			}
			return SexpArray(ret), nil
		}
		case "cvert-float32": {
			var ret []Sexp
			for i, arg := range args {
				switch t := arg.(type) {
					case SexpData: {
						buffer := bytes.NewBuffer([]byte(t))
						var value float32
						err := binary.Read(buffer, binary.LittleEndian, &value)
						if err != nil {
							return SexpNull, fmt.Errorf("%T: failed converting %v arg into int; %v", arg, i, err)
						}
						ret = append(ret, SexpFloat(value))
					}
					case SexpStr: {
						val, err := strconv.ParseFloat(string(t), 32)
						if err != nil {
							return SexpNull, fmt.Errorf("%T: failed converting %v arg into int; %v", arg, i, err)
						}
						ret = append(ret, SexpFloat(val))
					}
					case SexpInt: {
						ret = append(ret, SexpFloat(float64(int(t))))
					}
					case SexpBool: {
						if bool(t) {
							ret = append(ret, SexpFloat(1))
						} else {
							ret = append(ret, SexpFloat(0))
						}
					}
					default:
						return SexpNull, fmt.Errorf("%v unable to convert arg %v into int; unimplemented", name, i)
				}
			}
			return SexpArray(ret), nil
		}
		case "cvert-float64": {
			var ret []Sexp
			for i, arg := range args {
				switch t := arg.(type) {
					case SexpData: {
						buffer := bytes.NewBuffer([]byte(t))
						var value float64
						err := binary.Read(buffer, binary.LittleEndian, &value)
						if err != nil {
							return SexpNull, fmt.Errorf("%T: failed converting %v arg into int; %v", arg, i, err)
						}
						ret = append(ret, SexpFloat(value))
					}
					case SexpStr: {
						val, err := strconv.ParseFloat(string(t), 64)
						if err != nil {
							return SexpNull, fmt.Errorf("%T: failed converting %v arg into int; %v", arg, i, err)
						}
						ret = append(ret, SexpFloat(val))
					}
					case SexpInt: {
						ret = append(ret, SexpFloat(float64(int(t))))
					}
					case SexpBool: {
						if bool(t) {
							ret = append(ret, SexpFloat(1))
						} else {
							ret = append(ret, SexpFloat(0))
						}
					}
					default:
						return SexpNull, fmt.Errorf("%v unable to convert arg %v into int; unimplemented", name, i)
				}
			}
			return SexpArray(ret), nil
		}
	}

	return SexpNull, fmt.Errorf("Failure of `%v` function, not implemented")
}

func StringifyFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}

	return SexpStr(args[0].SexpString()), nil
}

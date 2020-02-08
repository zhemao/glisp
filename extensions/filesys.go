package glispext

import (
	"fmt"
	"github.com/zhemao/glisp/interpreter"
	"os"
	"errors"
	"path/filepath"
	"io/ioutil"
	"io"
)

func currentDir(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	dir, err := os.Getwd()
	if err != nil {
		return glisp.SexpNull, err
	}
	return glisp.SexpStr(dir), nil
}

func changeDir(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	if len(args) != 1 {
		return glisp.SexpNull, glisp.WrongNargs
	}

	var err error

	switch t := args[0].(type) {
	case glisp.SexpStr:
		err = os.Chdir(string(t))
	default:
		return glisp.SexpNull, fmt.Errorf("argument to %s must be string", name)
	}

	if err == nil {
		return glisp.SexpBool(true), nil
	} else {
		return glisp.SexpStr(err.Error()), nil
	}
}

func readDir(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	var err error

	var path string

	if pathA, ok := args[0].(glisp.SexpStr); ok {
		path = string(pathA)
	}

	if path == "" {
		path, err = os.Getwd()
		if err != nil {
			return glisp.SexpNull, err
		}
	}
	
	infos, err := ioutil.ReadDir(path)
	if err != nil {
		return glisp.SexpNull, err
	}

	var ret glisp.SexpArray

	for _, info := range infos {
		ginfo, _ := glisp.MakeHash(nil, "FileInfo")
		
		ginfo.HashSet(glisp.SexpStr("path"), glisp.SexpStr(path))
		ginfo.HashSet(glisp.SexpStr("name"), glisp.SexpStr(info.Name()))
		ginfo.HashSet(glisp.SexpStr("size"), glisp.SexpInt(info.Size()))
		ginfo.HashSet(glisp.SexpStr("mode"), glisp.SexpInt(info.Mode()))
		ginfo.HashSet(glisp.SexpStr("isdir"), glisp.SexpBool(info.IsDir()))

		ret = append(ret, ginfo)
	}

	return ret, nil
}

var abort error = errors.New("abort")

func walkDir(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	if len(args) != 1 {
		return glisp.SexpNull, glisp.WrongNargs
	}

	var fun glisp.SexpFunction

	switch t := args[0].(type) {
	case glisp.SexpFunction:
		fun = t
	default:
		return glisp.SexpNull, fmt.Errorf("argument to %s must be a `fun [fileInfo]`", name)
	}

	dir, err := os.Getwd()
	if err != nil {
		return glisp.SexpNull, err
	}

	err = filepath.Walk(dir, func (path string, info os.FileInfo, err error) error {

		ginfo, _ := glisp.MakeHash(nil, "FileInfo")
		
		ginfo.HashSet(glisp.SexpStr("path"), glisp.SexpStr(path))
		ginfo.HashSet(glisp.SexpStr("name"), glisp.SexpStr(info.Name()))
		ginfo.HashSet(glisp.SexpStr("size"), glisp.SexpInt(info.Size()))
		ginfo.HashSet(glisp.SexpStr("mode"), glisp.SexpInt(info.Mode()))
		ginfo.HashSet(glisp.SexpStr("isdir"), glisp.SexpBool(info.IsDir()))

		fnRet, err1 := env.Apply(fun, []glisp.Sexp{ginfo})

		if err1 != nil {
			return err1
		}

		if abrt, ok := fnRet.(glisp.SexpBool); ok && abrt == glisp.SexpBool(true) {
			return abort
		}

		return nil
	})

	if err != nil && err != abort {
		return glisp.SexpBool(false), err
	}

	return glisp.SexpBool(true), err
}

func pathSplit(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	if len(args) != 1 {
		return glisp.SexpNull, glisp.WrongNargs
	}

	var str glisp.SexpStr

	switch t := args[0].(type) {
	case glisp.SexpStr:
		str = t
	default:
		return glisp.SexpNull, fmt.Errorf("argument to %v must be a `string`", name)
	}

	var ret glisp.SexpArray

	var lastFront string

	for front, back := filepath.Split(string(str)); front != lastFront; front, back = filepath.Split(filepath.Dir(front)) {
		a := glisp.SexpStr(back)
		ret = append(ret, a)
		copy(ret[1:], ret[0:len(ret)-1])
		ret[0] = a
		lastFront = front
	}

	a := glisp.SexpStr(lastFront)
	ret = append(ret, a)
	copy(ret[1:], ret[0:len(ret)-1])
	ret[0] = a

	return ret, nil
}


func joinP(i int, combine string, arg glisp.Sexp) (string, error) {
	var err error

	switch t := arg.(type) {
		case glisp.SexpStr:
			combine = filepath.Join(combine, string(t))
		case glisp.SexpArray:
			for _, v := range t {
				combine, err = joinP(i, combine, v)
				if err != nil {
					return "", err
				}
			}
		default:
			return "",  fmt.Errorf("Invalid %v arg, requires string|array got => %T", i, arg)
	}
	return combine, nil
}

func pathJoin(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	combine := ""

	var err error

	for i, v := range args {
		combine, err = joinP(i, combine, v)
		if err != nil {
			return nil, err
		}
	}

	return glisp.SexpStr(combine), nil
}

func readFile(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	if len(args) < 1 {
		return glisp.SexpNull, glisp.WrongNargs
	}

	fileName, ok := args[0].(glisp.SexpStr)
	if !ok {
		return glisp.SexpNull, fmt.Errorf("expected `string` got %T; for arg 0 (filename)", args[0])
	}

	var offset int64
	if len(args) > 1 {
		o, ok := args[1].(glisp.SexpInt)
		if !ok {
			return glisp.SexpNull, fmt.Errorf("expected `int` got %T; for arg 1 (offset)", args[1])
		}
		offset = int64(o)
	}

	var max int64
	if len(args) > 2 {
		m, ok := args[2].(glisp.SexpInt)
		if !ok {
			return glisp.SexpNull, fmt.Errorf("expected `int` got %T; for arg 2 (max)", args[2])
		}
		max = int64(m)
	}

	var err error
	
	stat, err := os.Stat(string(fileName))
	if err != nil {
		return glisp.SexpNull, err
	}

	if stat.Size() < offset + max || max == 0 {
		max = stat.Size() - offset
	}

	f, err := os.Open(string(fileName))	
	if err != nil {
		return glisp.SexpNull, err
	}

	defer func () {
		f.Close()
	}()

	_, err = f.Seek(offset, 0)
	if err != nil {
		return glisp.SexpNull, err
	}

	buf := make([]byte, max)
	n, err := f.Read(buf)
	if err != nil {
		return glisp.SexpNull, err
	}

	return glisp.SexpData(buf[0:n]), nil
}

// (fs-read-file-s <filename> <fn [pos data]> <chunkSz> [offset] [max])
func readStreamFile(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	if len(args) < 3 {
		return glisp.SexpNull, glisp.WrongNargs
	}

	fileName, ok := args[0].(glisp.SexpStr)
	if !ok {
		return glisp.SexpNull, fmt.Errorf("expected `string` got %T; for arg 0 (filename)", args[0])
	}

	fun, ok := args[1].(glisp.SexpFunction)
	if !ok {
		return glisp.SexpNull, fmt.Errorf("expected `function` got %T; for arg 1 (stream-fn)", args[1])
	}

	chunk, ok := args[2].(glisp.SexpInt)
	if !ok {
		return glisp.SexpNull, fmt.Errorf("expected `int` got %T; for arg 2 (chunk-size)", args[2])
	}

	var offset int64
	if len(args) > 3 {
		o, ok := args[3].(glisp.SexpInt)
		if !ok {
			return glisp.SexpNull, fmt.Errorf("expected `int` got %T; for arg 3 (offset)", args[3])
		}
		offset = int64(o)
	}

	var max int64
	if len(args) > 4 {
		m, ok := args[4].(glisp.SexpInt)
		if !ok {
			return glisp.SexpNull, fmt.Errorf("expected `int` got %T; for arg 4 (max)", args[4])
		}
		max = int64(m)
	}

	var err error
	
	stat, err := os.Stat(string(fileName))
	if err != nil {
		return glisp.SexpNull, err
	}

	if stat.Size() < offset + max || max == 0 {
		max = stat.Size() - offset
	}

	f, err := os.Open(string(fileName))	
	if err != nil {
		return glisp.SexpNull, err
	}

	defer func () {
		f.Close()
	}()

	pos, err := f.Seek(offset, 0)
	if err != nil {
		return glisp.SexpNull, err
	}

	buf := make([]byte, chunk)

	for pos < max {
		n, err := f.Read(buf)

		if err != nil && err != io.EOF {
			return glisp.SexpNull, err
		}

		if n > 0 {
			fnRet, err1 := env.Apply(fun, []glisp.Sexp{glisp.SexpInt(pos), glisp.SexpData(buf[0:n])})
			if err1 != nil {
				return nil, err1
			}

			if abort, ok := fnRet.(glisp.SexpBool); ok && bool(abort) {
				break
			}
		}


		if err == io.EOF {
			break
		}

		pos += int64(n)
	}


	return glisp.SexpInt(int(pos - offset)), nil
}

// (fs-append-file-s <filename> <fn [pos] => (data)>)
func appendStreamFile(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	if len(args) < 2 {
		return glisp.SexpNull, glisp.WrongNargs
	}

	fileName, ok := args[0].(glisp.SexpStr)
	if !ok {
		return glisp.SexpNull, fmt.Errorf("expected `string` got %T; for arg 0 (filename)", args[0])
	}

	fun, ok := args[1].(glisp.SexpFunction)
	if !ok {
		return glisp.SexpNull, fmt.Errorf("expected `function` got %T; for arg 1 (stream-fn)", args[1])
	}

	f, err := os.OpenFile(string(fileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)	
	if err != nil {
		return glisp.SexpNull, err
	}

	defer func () {
		f.Close()
	}()

	pos, err := f.Seek(0, 2)
	if err != nil {
		return glisp.SexpNull, err
	}

	for {
		fnRet, err := env.Apply(fun, []glisp.Sexp{glisp.SexpInt(pos)})
		if err != nil {
			return nil, err
		}


		data, ok := fnRet.(glisp.SexpData)
		if !ok {
			return nil, fmt.Errorf("stream funciton return something other then `data` aborting")
		}

		if len([]byte(data)) == 0 {
			break
		}

		n, err := f.Write(data)
		if n < len(data) {
			return nil, fmt.Errorf("trying to write data(len %v) failed only wrote %v, aborting", len(data), n)
		}
		if err != nil {
			return nil, err
		}

		pos += int64(n)
	}

	return glisp.SexpInt(pos), nil
}

// (fs-append-file <filename> <data> [<data>...])
func appendFile(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	if len(args) < 2 {
		return glisp.SexpNull, glisp.WrongNargs
	}

	fileName, ok := args[0].(glisp.SexpStr)
	if !ok {
		return glisp.SexpNull, fmt.Errorf("expected `string` got %T; for arg 0 (filename)", args[0])
	}

	f, err := os.OpenFile(string(fileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)	
	if err != nil {
		return glisp.SexpNull, err
	}

	defer func () {
		f.Close()
	}()

	pos, err := f.Seek(0, 2)
	if err != nil {
		return glisp.SexpNull, err
	}

	for i, arg := range args[1:] {
		data, ok := arg.(glisp.SexpData)
		if !ok {
			return glisp.SexpNull, fmt.Errorf("expected `data` got %T; for arg %v (data)", data, i+1)
		}

		n, err := f.Write(data)
		if n < len(data) {
			return nil, fmt.Errorf("trying to write data(len %v) failed only wrote %v, aborting @pos %v", len(data), n, pos)
		}
		if err != nil {
			return nil, err
		}
		pos += int64(n)
	}


	return glisp.SexpInt(pos), nil
}

func removeFile(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	for i, arg := range args {
		file, ok := arg.(glisp.SexpStr)
		if !ok {
			return glisp.SexpNull, fmt.Errorf("invalid arg(%v) %T passed, expected string", i, arg)
		}

		err := os.Remove(string(file))
		if err != nil {
			return glisp.SexpNull, fmt.Errorf("arg(%v); error removing file %v; err %v", i, arg, err)
		}
	}
	return glisp.SexpNull, nil
}

func fileExists(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	for i, arg := range args {
		file, ok := arg.(glisp.SexpStr)
		if !ok {
			return glisp.SexpNull, fmt.Errorf("invalid arg(%v) %T passed, expected string", i, arg)
		}

		stat, err := os.Stat(string(file))
		if os.IsNotExist(err) {
			return glisp.SexpBool(false), nil
		}

		if err != nil {
			return glisp.SexpNull, fmt.Errorf("arg(%v); testing exists file %v; err %v", i, arg, err)
		}

		if stat == nil {
			return glisp.SexpBool(false), nil
		}
	}
	return glisp.SexpBool(true), nil
}

func fileInfo(env *glisp.Glisp, name string, args []glisp.Sexp) (glisp.Sexp, error) {
	for i, arg := range args {
		file, ok := arg.(glisp.SexpStr)
		if !ok {
			return glisp.SexpNull, fmt.Errorf("invalid arg(%v) %T passed, expected string", i, arg)
		}

		info, err := os.Stat(string(file))

		ginfo, _ := glisp.MakeHash(nil, "FileInfo")

		if os.IsNotExist(err) {
			ginfo.HashSet(glisp.SexpStr("exists"), glisp.SexpBool(false))
			ginfo.HashSet(glisp.SexpStr("path"), glisp.SexpStr(""))
			ginfo.HashSet(glisp.SexpStr("name"), glisp.SexpStr(""))
			ginfo.HashSet(glisp.SexpStr("size"), glisp.SexpInt(0))
			ginfo.HashSet(glisp.SexpStr("mode"), glisp.SexpInt(0))
			ginfo.HashSet(glisp.SexpStr("isdir"), glisp.SexpBool(false))
			return ginfo, nil
		}

		
		if err != nil {
			return glisp.SexpNull, fmt.Errorf("arg(%v); testing exists file %v; err %v", i, arg, err)
		}

		ginfo.HashSet(glisp.SexpStr("exists"), glisp.SexpBool(true))
		ginfo.HashSet(glisp.SexpStr("path"), glisp.SexpStr(file))
		ginfo.HashSet(glisp.SexpStr("name"), glisp.SexpStr(info.Name()))
		ginfo.HashSet(glisp.SexpStr("size"), glisp.SexpInt(info.Size()))
		ginfo.HashSet(glisp.SexpStr("mode"), glisp.SexpInt(info.Mode()))
		ginfo.HashSet(glisp.SexpStr("isdir"), glisp.SexpBool(info.IsDir()))

		return ginfo, nil
	}
	return glisp.SexpNull, nil
}

func ImportFileSys(env *glisp.Glisp) {
	env.AddFunction("fs-cwd", currentDir)
	env.AddFunction("fs-chdir", changeDir)
	env.AddFunction("fs-walk", walkDir)
	env.AddFunction("fs-readdir", readDir)
	env.AddFunction("fs-path-split", pathSplit)
	env.AddFunction("fs-path-join", pathJoin)
	env.AddFunction("fs-file-exists", fileExists)
	env.AddFunction("fs-file-info", fileInfo)
	env.AddFunction("fs-read-file", readFile)
	env.AddFunction("fs-read-file-s", readStreamFile)
	env.AddFunction("fs-remove-file", removeFile)
	env.AddFunction("fs-append-file-s", appendStreamFile)
	env.AddFunction("fs-append-file", appendFile)
}

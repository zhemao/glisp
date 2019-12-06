package glispext

import (
	"fmt"
	"github.com/zhemao/glisp/interpreter"
	"os"
	"errors"
	"path/filepath"
	"io/ioutil"
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

func ImportFileSys(env *glisp.Glisp) {
	env.AddFunction("fs-cwd", currentDir)
	env.AddFunction("fs-chdir", changeDir)
	env.AddFunction("fs-walk", walkDir)
	env.AddFunction("fs-readdir", readDir)
	env.AddFunction("fs-path-split", pathSplit)
	env.AddFunction("fs-path-join", pathJoin)
}

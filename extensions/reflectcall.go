package glispext

import (
	"fmt"
	"reflect"
	"regexp"
	"time"
)

// spike: how to do reflective function calls in go, not really integrated into glisp at this point.

type Controller struct{}

var count int = 0

func (c *Controller) Root(params map[string][]string) map[string]string {
	//fmt.Printf("Controler::Root() called!\n")
	count++
	return map[string]string{}
}

// speed of calls by reflection?
func mainEx() {

	controller_ref := Controller{}
	action_name := "Root"

	//	func (v Value) Call(in []Value) []Value
	m := map[string][]string{"foo": []string{"bar"}}

	in := []reflect.Value{reflect.ValueOf(m)}

	// on OSX, we get about 1239157 calls per second, or 8e-7 sec/call.
	//  (800 nanoseconds/call, round up to 1 usec/call)
	start := time.Now()
	N := 100
	for i := 0; i < N; i++ {
		myMethod := reflect.ValueOf(&controller_ref).MethodByName(action_name)
		myMethod.Call(in)
	}
	fmt.Printf("elap: %v\n", time.Since(start))

	call_lib()
}

var rxBool = regexp.MustCompile("^(true|false)$")

func call_lib() {

	atom := "false"
	if rxBool.MatchString(atom) {
		fmt.Printf("atom is bool\n")
	}

	matchString := reflect.ValueOf(regexp.MatchString)
	fmt.Printf("matchString = '%v'\n", matchString)

	m := "match against me? false"

	in := []reflect.Value{reflect.ValueOf("^(true|false)$"), reflect.ValueOf(m)}
	out := matchString.Call(in)
	fmt.Printf("out = %#v\n", out)

	var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

	matched := out[0].Bool()
	err := out[1].Convert(typeOfError).Interface()
	switch err.(type) {
	case nil:
		fmt.Printf("err is nil\n")
	case error:
		fmt.Printf("err is not nil!, is '%s'\n", err.(error))
		err = err.(error)
	}

	fmt.Printf("matched = '%v', err = '%v'\n", matched, err)

	/*
		controller_ref := Controller{}
		//func MustCompile(str string) *Regexp
		action_name := "regexp.MustCompile"

		//	func (v Value) Call(in []Value) []Value
		m := map[string][]string{"foo": []string{"bar"}}

		in := []reflect.Value{reflect.ValueOf(m)}

		// we get about 1239157 calls per second, or 8e-7 sec/call. (800 nanoseconds/call, round up to 1 usec/call)
		start := time.Now()
		N := 100
		for i := 0; i < N; i++ {
			myMethod := reflect.ValueOf(&controller_ref).MethodByName(action_name)
			myMethod.Call(in)
		}
		fmt.Printf("elap: %v\n", time.Since(start))
	*/
}

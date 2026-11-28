package ixgo

import (
	"fmt"
	"testing"

	"golang.org/x/tools/go/ssa"
)

func TestClosureElisionDefaultBackend(t *testing.T) {
	const source = `package main
import "runtime"
func Run()(result int) {
 defer func(){runtime.GC();result++}()
 for j:=0;j<3;j++ {
  p:=new(int);*p=j
  f:=func()int{runtime.GC();return *p}
  result+=f()
  defer func(){runtime.GC();result+=*p}()
 }
 return
}`
	for _, mode := range []Mode{0, ExperimentalSupportGC, ExperimentalSupportGC | EnableGCExecutionPlan} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			i, err := NewContext(DisableMemo|mode).LoadInterp("default_closure.go", source)
			if err != nil {
				t.Fatal(err)
			}
			defer i.UnsafeRelease()
			for n := 0; n < 3; n++ {
				got, err := i.RunFunc("Run")
				if err != nil || got != 7 {
					t.Fatalf("Run=%v,%v; want 7", got, err)
				}
			}
			count := 0
			for _, p := range i.funcs {
				for _, block := range p.Fn.Blocks {
					for _, instr := range block.Instrs {
						closure, ok := instr.(*ssa.MakeClosure)
						if !ok {
							continue
						}
						count++
						if _, ok := p.index[closure]; ok {
							t.Fatalf("%s: call-only closure still owns a slot", p.Fn)
						}
					}
				}
			}
			if count < 3 {
				t.Fatalf("only %d closure fixtures", count)
			}
		})
	}
}

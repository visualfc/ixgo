package ixgo

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

func skipBlankFieldStore(store *ssa.Store) bool {
	addr, ok := store.Addr.(*ssa.FieldAddr)
	if !ok {
		return false
	}
	ptr, ok := addr.X.Type().Underlying().(*types.Pointer)
	if !ok {
		return false
	}
	fields, ok := ptr.Elem().Underlying().(*types.Struct)
	return ok && fields.Field(addr.Field).Name() == "_"
}

func skipMissingInit(interp *Interp, fn *ssa.Function) bool {
	if fn.Blocks != nil || fn.Pkg == nil || fn.Name() != "init" {
		return false
	}
	// A direct adapter can provide an initializer without a reflected function.
	if _, ok := staticDirectAdapter(interp, fn); ok {
		return false
	}
	_, ok := findExternFunc(interp, fn)
	return !ok
}

func skipArrayRangeDeref(load *ssa.UnOp) bool {
	return skipUnusedArrayDeref(load) && !arrayPointerOperandHasEffectAfter(load.X, load.Pos(), nil)
}

// One Phi executor performs the complete parallel assignment.
func skipNonleadingPhi(phi *ssa.Phi) bool {
	return phi.Block().Instrs[0] != phi
}

func recordPackageInitStore(pfn *function, store *ssa.Store) {
	if pfn.Fn.Name() == "init" && pfn.Fn.Synthetic == "package initializer" {
		pfn.Interp.chkinit[store.Addr.String()] = true
	}
}

// skipCallOnlyClosure omits function values used only for known-body calls or
// defers. Defer preparation snapshots bindings before returning to the caller.
// Other uses, including Go and DebugRef, keep the function value observable.
func skipCallOnlyClosure(closure *ssa.MakeClosure) bool {
	refs := closure.Referrers()
	if refs == nil || len(*refs) == 0 {
		return false
	}
	for _, ref := range *refs {
		var call *ssa.CallCommon
		switch ref := ref.(type) {
		case *ssa.Call:
			call = &ref.Call
		case *ssa.Defer:
			call = &ref.Call
		default:
			return false
		}
		if call.Value != closure {
			return false
		}
		for _, arg := range call.Args {
			if arg == closure {
				return false
			}
		}
	}
	return true
}

// closureLivenessOperands exposes captured values read by calls whose closure
// target has no frame slot. Materialized closures use the capture graph instead.
func closureLivenessOperands(instr ssa.Instruction, buf []*ssa.Value) []*ssa.Value {
	ops := instr.Operands(buf)
	count := len(ops)
	for _, op := range ops[:count] {
		if op == nil {
			continue
		}
		if closure, ok := (*op).(*ssa.MakeClosure); ok && skipCallOnlyClosure(closure) {
			for i := range closure.Bindings {
				ops = append(ops, &closure.Bindings[i])
			}
		}
	}
	return ops
}
